package external

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/navidrome/navidrome/core/storage/remoteio"
	sources "github.com/navidrome/navidrome/integrations/external_sources"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

var sftpSlots = make(chan struct{}, 8)

type sftpBackend struct {
	s      sources.Source
	root   string
	secret credentials
	dial   func(context.Context, string, string) (net.Conn, error)
}

func newSFTP(s sources.Source, root string, secret credentials, dial func(context.Context, string, string) (net.Conn, error)) (remoteio.Backend, error) {
	if secret.Username == "" || secret.Password == "" && secret.PrivateKey == "" {
		return nil, errors.New("SFTP credentials missing")
	}
	return &sftpBackend{s, root, secret, dial}, nil
}
func (b *sftpBackend) run(ctx context.Context, fn func(*sftp.Client) error) error {
	select {
	case sftpSlots <- struct{}{}:
		defer func() { <-sftpSlots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	auth := []ssh.AuthMethod{}
	if b.secret.Password != "" {
		auth = append(auth, ssh.Password(b.secret.Password))
	}
	if b.secret.PrivateKey != "" {
		signer, e := ssh.ParsePrivateKey([]byte(b.secret.PrivateKey))
		if e != nil {
			return errors.New("invalid SFTP key")
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	config := &ssh.ClientConfig{User: b.secret.Username, Auth: auth, Timeout: 15 * time.Second, HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
		if ssh.FingerprintSHA256(key) != b.s.HostKeySHA256 {
			return errors.New("SFTP host key mismatch")
		}
		return nil
	}}
	u, _ := url.Parse(b.s.Endpoint)
	addr := u.Host
	if u.Port() == "" {
		addr = net.JoinHostPort(u.Hostname(), "22")
	}
	c, e := b.dial(ctx, "tcp", addr)
	if e != nil {
		return errors.New("SFTP unavailable")
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(30 * time.Second))
	conn, ch, req, e := ssh.NewClientConn(c, addr, config)
	if e != nil {
		return errors.New("SFTP handshake refused")
	}
	sshClient := ssh.NewClient(conn, ch, req)
	defer sshClient.Close()
	client, e := sftp.NewClient(sshClient, sftp.MaxConcurrentRequestsPerFile(1))
	if e != nil {
		return errors.New("SFTP subsystem unavailable")
	}
	defer client.Close()
	if e = fn(client); e != nil {
		return errors.New("SFTP read failed")
	}
	return nil
}
func (b *sftpBackend) jailed(c *sftp.Client, name string) (string, error) {
	root, e := c.RealPath("/" + b.root)
	if e != nil {
		return "", e
	}
	want := path.Join(root, name)
	real, e := c.RealPath(want)
	if e != nil {
		return "", e
	}
	if real != root && !strings.HasPrefix(real, strings.TrimSuffix(root, "/")+"/") {
		return "", fs.ErrPermission
	}
	return real, nil
}
func (b *sftpBackend) List(ctx context.Context, name string) (out []fs.DirEntry, err error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	out = []fs.DirEntry{}
	err = b.run(ctx, func(c *sftp.Client) error {
		p, e := b.jailed(c, name)
		if e != nil {
			return e
		}
		entries, e := c.ReadDir(p)
		if e != nil {
			return e
		}
		if len(entries) > 100000 {
			return errors.New("SFTP directory bound exceeded")
		}
		for _, v := range entries {
			if v.Mode()&fs.ModeSymlink != 0 {
				continue
			}
			if !v.IsDir() && !v.Mode().IsRegular() {
				continue
			}
			out = append(out, remoteio.Entry{Filename: v.Name(), Length: v.Size(), Directory: v.IsDir(), Modified: v.ModTime()})
		}
		return nil
	})
	return
}
func (b *sftpBackend) Stat(ctx context.Context, name string) (result fs.FileInfo, err error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	err = b.run(ctx, func(c *sftp.Client) error {
		canonical, e := b.jailed(c, name)
		if e != nil {
			return e
		}
		info, e := c.Stat(canonical)
		if e != nil {
			return e
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fs.ErrInvalid
		}
		version := "\"" + hash(fmt.Sprintf("%s:%d:%d", canonical, info.Size(), info.ModTime().UnixNano())) + "\""
		result = remoteio.Entry{Filename: path.Base(name), Length: info.Size(), Directory: info.IsDir(), Modified: info.ModTime(), Version: version}
		return nil
	})
	return
}

func (b *sftpBackend) Range(ctx context.Context, name, ran, etag string) (response *http.Response, err error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	var start, end int64
	if _, e := fmt.Sscanf(ran, "bytes=%d-%d", &start, &end); e != nil || start < 0 || end < start || end-start >= 256<<10 {
		return nil, fs.ErrInvalid
	}
	err = b.run(ctx, func(c *sftp.Client) error {
		p, e := b.jailed(c, name)
		if e != nil {
			return e
		}
		f, e := c.Open(p)
		if e != nil {
			return e
		}
		defer f.Close()
		info, e := f.Stat()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() || end >= info.Size() {
			return fs.ErrInvalid
		}
		version := "\"" + hash(fmt.Sprintf("%s:%d:%d", p, info.Size(), info.ModTime().UnixNano())) + "\""
		if etag != "" && etag != version {
			return errors.New("SFTP file changed")
		}
		data := make([]byte, end-start+1)
		if _, e = f.ReadAt(data, start); e != nil {
			return e
		}
		after, e := f.Stat()
		if e != nil || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
			return errors.New("SFTP file changed")
		}
		response = &http.Response{StatusCode: 206, Header: http.Header{"Etag": {version}, "Content-Range": {fmt.Sprintf("bytes %d-%d/%d", start, end, info.Size())}}, Body: io.NopCloser(bytes.NewReader(data))}
		return nil
	})
	return
}
