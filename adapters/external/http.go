package external

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"

	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"

	"github.com/navidrome/navidrome/core/storage/remoteio"
	sources "github.com/navidrome/navidrome/integrations/external_sources"
	"github.com/navidrome/navidrome/utils/netguard"
	"golang.org/x/net/proxy"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"

	"strconv"
	"strings"
	"sync"
	"time"
)

// Bound active and idle external-source sockets across all MusicFS instances.
var externalSockets = make(chan struct{}, 64)

type countedConn struct {
	net.Conn
	once sync.Once
}

func (c *countedConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { <-externalSockets })
	return err
}

type credentials struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	AccessKey    string `json:"access_key"`
	SecretKey    string `json:"secret_key"`
	SessionToken string `json:"session_token"`
	PrivateKey   string `json:"private_key"`
}
type httpBackend struct {
	s      sources.Source
	root   string
	origin *url.URL
	client *http.Client
	secret credentials
}

func allowedIP(ip net.IP, s sources.Source) bool {
	_, cgn, _ := net.ParseCIDR("100.64.0.0/10")
	if !netguard.IsPrivateIP(ip) && !cgn.Contains(ip) && !ip.IsUnspecified() && !ip.IsMulticast() {
		return true
	}
	for _, v := range s.AllowedNetworks {
		_, n, e := net.ParseCIDR(v)
		if e == nil && n.Contains(ip) {
			return true
		}
	}
	return false
}
func pinnedDial(s sources.Source) (func(context.Context, string, string) (net.Conn, error), error) {
	u, e := url.Parse(s.Endpoint)
	if e != nil {
		return nil, fs.ErrInvalid
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "443"
		if s.Type == "sftp" {
			port = "22"
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
	if e != nil || len(ips) == 0 {
		return nil, errors.New("source DNS unavailable")
	}
	for _, ip := range ips {
		if !allowedIP(ip.IP, s) {
			return nil, errors.New("source resolves to unapproved network")
		}
	}
	var connector proxy.ContextDialer = &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	if s.SocksProxyEnv != "" {
		var config struct {
			Address string `json:"address"`
		}
		if json.Unmarshal([]byte(os.Getenv(s.SocksProxyEnv)), &config) != nil {
			return nil, errors.New("invalid private SOCKS proxy configuration")
		}
		host, _, err := net.SplitHostPort(config.Address)
		if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
			return nil, errors.New("SOCKS proxy must bind a loopback IP")
		}
		d, err := proxy.SOCKS5("tcp", config.Address, nil, &net.Dialer{Timeout: 10 * time.Second})
		if err != nil {
			return nil, errors.New("private SOCKS proxy unavailable")
		}
		connector = d.(proxy.ContextDialer)
	}
	target := net.JoinHostPort(ips[0].IP.String(), port)
	expected := net.JoinHostPort(host, port)
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != expected {
			return nil, errors.New("unapproved destination")
		}
		select {
		case externalSockets <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		conn, err := connector.DialContext(ctx, network, target)
		if err != nil {
			<-externalSockets
			return nil, err
		}
		return &countedConn{Conn: conn}, nil
	}, nil
}
func NewBackend(s sources.Source, suffix string) (remoteio.Backend, error) {
	if e := (sources.Config{Sources: []sources.Source{s}}).Validate(); e != nil {
		return nil, e
	}
	if suffix != "" && (!fs.ValidPath(suffix) || strings.Contains(suffix, "\\")) {
		return nil, fs.ErrInvalid
	}
	var secret credentials
	if e := json.Unmarshal([]byte(os.Getenv(s.SecretEnv)), &secret); e != nil {
		return nil, errors.New("source credential unavailable")
	}
	if s.Type == "s3" && (secret.AccessKey == "" || secret.SecretKey == "") {
		return nil, errors.New("S3 credentials missing")
	}
	dial, e := pinnedDial(s)
	if e != nil {
		return nil, e
	}
	root := path.Join(s.Root, suffix)
	if root == "." {
		root = ""
	}
	if s.Type == "sftp" {
		return newSFTP(s, root, secret, dial)
	}
	u, _ := url.Parse(s.Endpoint)
	tr := &http.Transport{DialContext: dial, MaxConnsPerHost: 4, MaxIdleConnsPerHost: 2, MaxIdleConns: 2, IdleConnTimeout: 15 * time.Second, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 20 * time.Second, DisableCompression: true}
	return &httpBackend{s: s, root: root, origin: u, secret: secret, client: &http.Client{Transport: tr, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (b *httpBackend) request(ctx context.Context, method, name string, query url.Values, body io.Reader, headers http.Header) (*http.Response, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	u := *b.origin
	if b.s.Type == "s3" {
		u.Path = "/" + b.s.Bucket
	} else {
		u.Path = ""
	}
	if name != "." {
		u.Path += "/" + name
	} else {
		u.Path += "/"
	}
	u.RawQuery = query.Encode()
	req, e := http.NewRequestWithContext(ctx, method, u.String(), body)
	if e != nil {
		return nil, e
	}
	req.Header = headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set("Accept-Encoding", "identity")
	if b.s.Type == "s3" {
		if e := b.sign(req); e != nil {
			return nil, errors.New("S3 signing failed")
		}
	} else {
		req.SetBasicAuth(b.secret.Username, b.secret.Password)
	}
	r, e := b.client.Do(req)
	if e != nil {
		return nil, errors.New("external request failed")
	}
	return r, nil
}
func boundedXML(r *http.Response, out any) error {
	defer r.Body.Close()
	data, e := io.ReadAll(io.LimitReader(r.Body, (8<<20)+1))
	if e != nil || len(data) > 8<<20 {
		return errors.New("listing too large or incomplete")
	}
	return xml.Unmarshal(data, out)
}
func (b *httpBackend) Range(ctx context.Context, name, ran, etag string) (*http.Response, error) {
	h := http.Header{"Range": {ran}}
	if etag != "" {
		h.Set("If-Match", etag)
	}
	return b.request(ctx, "GET", path.Join(b.root, name), nil, nil, h)
}
func (b *httpBackend) List(ctx context.Context, name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	if b.s.Type == "s3" {
		return b.listS3(ctx, name)
	}
	return b.listDAV(ctx, name)
}
func (b *httpBackend) listDAV(ctx context.Context, name string) ([]fs.DirEntry, error) {
	dir := path.Join(b.root, name)
	r, e := b.request(ctx, "PROPFIND", dir, nil, strings.NewReader(`<?xml version="1.0"?><propfind xmlns="DAV:"><prop><resourcetype/><getcontentlength/><getlastmodified/><getetag/></prop></propfind>`), http.Header{"Depth": {"1"}, "Content-Type": {"application/xml"}})
	if e != nil {
		return nil, e
	}
	if r.StatusCode != 207 {
		r.Body.Close()
		return nil, errors.New("WebDAV directory unavailable")
	}
	var data struct {
		XMLName   xml.Name `xml:"DAV: multistatus"`
		Responses []struct {
			Href  string `xml:"DAV: href"`
			Props []struct {
				Status string `xml:"DAV: status"`
				Prop   struct {
					Collection *struct{} `xml:"DAV: resourcetype>collection"`
					Size       string    `xml:"DAV: getcontentlength"`
					Modified   string    `xml:"DAV: getlastmodified"`
				} `xml:"DAV: prop"`
			} `xml:"DAV: propstat"`
		} `xml:"DAV: response"`
	}
	if e = boundedXML(r, &data); e != nil {
		return nil, e
	}
	if len(data.Responses) == 0 {
		return nil, errors.New("empty WebDAV multistatus")
	}
	parent := "/" + strings.Trim(dir, "/")
	if parent == "/." {
		parent = ""
	}
	out := []fs.DirEntry{}
	self := false
	for _, entry := range data.Responses {
		u, e := url.Parse(entry.Href)
		if e != nil || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return nil, fs.ErrInvalid
		}
		if u.IsAbs() && (u.Scheme != b.origin.Scheme || u.Host != b.origin.Host) {
			return nil, fs.ErrPermission
		}
		p := strings.TrimSuffix(u.Path, "/")
		if p == parent || p == "" && parent == "" {
			self = true
			continue
		}
		if path.Dir(p) != parent && !(parent == "" && path.Dir(p) == "/") {
			return nil, fs.ErrInvalid
		}
		item := remoteio.Entry{Filename: path.Base(p)}
		valid := false
		for _, prop := range entry.Props {
			if strings.Contains(prop.Status, " 200 ") {
				valid = true
				item.Directory = prop.Prop.Collection != nil
				if !item.Directory {
					item.Length, e = strconv.ParseInt(prop.Prop.Size, 10, 64)
					if e != nil || item.Length < 0 {
						return nil, fs.ErrInvalid
					}
				}
				item.Modified, _ = http.ParseTime(prop.Prop.Modified)
			}
		}
		if !valid {
			return nil, errors.New("incomplete WebDAV properties")
		}
		out = append(out, item)
	}
	if !self {
		return nil, errors.New("WebDAV root response missing")
	}
	return out, nil
}
func (b *httpBackend) listS3(ctx context.Context, name string) ([]fs.DirEntry, error) {
	prefix := path.Join(b.root, name)
	if prefix == "." {
		prefix = ""
	}
	if prefix != "" {
		prefix += "/"
	}
	out := []fs.DirEntry{}
	token := ""
	seen := map[string]bool{}
	for page := 0; page < 10000; page++ {
		q := url.Values{"list-type": {"2"}, "prefix": {prefix}, "delimiter": {"/"}, "max-keys": {"1000"}}
		if token != "" {
			q.Set("continuation-token", token)
		}
		r, e := b.request(ctx, "GET", ".", q, nil, nil)
		if e != nil {
			return nil, e
		}
		if r.StatusCode != 200 {
			r.Body.Close()
			return nil, errors.New("S3 listing unavailable")
		}
		var d struct {
			XMLName   xml.Name `xml:"ListBucketResult"`
			Truncated bool     `xml:"IsTruncated"`
			Next      string   `xml:"NextContinuationToken"`
			Contents  []struct {
				Key          string
				Size         int64
				LastModified string
			} `xml:"Contents"`
			Prefixes []struct{ Prefix string } `xml:"CommonPrefixes"`
		}
		if e = boundedXML(r, &d); e != nil {
			return nil, e
		}
		add := func(key string, dir bool, size int64, modified string) error {
			if key == prefix {
				return nil
			}
			if !strings.HasPrefix(key, prefix) || size < 0 {
				return fs.ErrInvalid
			}
			n := strings.TrimSuffix(strings.TrimPrefix(key, prefix), "/")
			if !fs.ValidPath(n) || strings.Contains(n, "/") || seen[n] {
				return fs.ErrInvalid
			}
			seen[n] = true
			t, _ := time.Parse(time.RFC3339Nano, modified)
			out = append(out, remoteio.Entry{Filename: n, Directory: dir, Length: size, Modified: t})
			return nil
		}
		for _, v := range d.Contents {
			if e = add(v.Key, false, v.Size, v.LastModified); e != nil {
				return nil, e
			}
		}
		for _, v := range d.Prefixes {
			if e = add(v.Prefix, true, 0, ""); e != nil {
				return nil, e
			}
		}
		if len(out) > 100000 {
			return nil, errors.New("directory exceeds entry bound")
		}
		if !d.Truncated {
			return out, nil
		}
		if d.Next == "" || d.Next == token {
			return nil, errors.New("incomplete S3 pagination")
		}
		token = d.Next
	}
	return nil, errors.New("S3 pagination exceeds bound")
}
func hash(data string) string { h := sha256.Sum256([]byte(data)); return hex.EncodeToString(h[:]) }
func (b *httpBackend) sign(r *http.Request) error {
	payload := hash("")
	r.Header.Set("X-Amz-Content-Sha256", payload)
	return v4.NewSigner().SignHTTP(r.Context(), aws.Credentials{AccessKeyID: b.secret.AccessKey, SecretAccessKey: b.secret.SecretKey, SessionToken: b.secret.SessionToken}, r, payload, "s3", b.s.Region, time.Now().UTC(), func(o *v4.SignerOptions) { o.DisableURIPathEscaping = true })
}

// Stat obtains current object length/version without downloading its contents.
func (b *httpBackend) Stat(ctx context.Context, name string) (fs.FileInfo, error) {
	r, e := b.request(ctx, "HEAD", path.Join(b.root, name), nil, nil, nil)
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 || r.ContentLength < 0 {
		return nil, errors.New("remote object stat unavailable")
	}
	etag := r.Header.Get("ETag")
	if etag == "" || strings.HasPrefix(etag, "W/") {
		return nil, errors.New("strong object ETag required")
	}
	modified, _ := http.ParseTime(r.Header.Get("Last-Modified"))
	return remoteio.Entry{Filename: path.Base(name), Length: r.ContentLength, Modified: modified, Version: etag}, nil
}
