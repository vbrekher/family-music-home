// Local-only read-only protocol fixture. No real NAS or production credentials.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	sources "github.com/navidrome/navidrome/integrations/external_sources"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type fixture struct {
	data []byte
	etag string
}

var files = map[string]fixture{}

func escape(s string) string { var b bytes.Buffer; xml.EscapeText(&b, []byte(s)); return b.String() }
func entries(root string) []string {
	if strings.HasPrefix(root, "scale") {
		n, _ := strconv.Atoi(strings.TrimPrefix(root, "scale"))
		if n > 10000 || n < 1 {
			return nil
		}
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("%05d.mp3", i)
		}
		return out
	}
	out := []string{}
	for n := range files {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
func get(root, name string) (fixture, bool) {
	if strings.HasPrefix(root, "scale") {
		n, e := strconv.Atoi(strings.TrimSuffix(name, ".mp3"))
		limit, _ := strconv.Atoi(strings.TrimPrefix(root, "scale"))
		if e != nil || n < 0 || n >= limit {
			return fixture{}, false
		}
		return files["scale-template.mp3"], true
	}
	f, ok := files[name]
	return f, ok
}
func serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" && r.Method != "PROPFIND" {
		http.Error(w, "read-only", 405)
		return
	}
	s3 := strings.HasPrefix(r.URL.Path, "/bucket/")
	if s3 {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=test/") {
			http.Error(w, "auth", 403)
			return
		}
	} else {
		u, p, ok := r.BasicAuth()
		if !ok || u != "test" || p != "test-only" {
			http.Error(w, "auth", 403)
			return
		}
	}
	if r.URL.Query().Get("list-type") == "2" {
		prefix := r.URL.Query().Get("prefix")
		root := strings.TrimSuffix(prefix, "/")
		names := entries(root)
		start, _ := strconv.Atoi(r.URL.Query().Get("continuation-token"))
		end := min(start+1000, len(names))
		if start < 0 || start > end {
			http.Error(w, "bad token", 400)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, "<ListBucketResult><IsTruncated>%t</IsTruncated>", end < len(names))
		if end < len(names) {
			fmt.Fprintf(w, "<NextContinuationToken>%d</NextContinuationToken>", end)
		}
		for _, name := range names[start:end] {
			f, _ := get(root, name)
			fmt.Fprintf(w, "<Contents><Key>%s</Key><Size>%d</Size><LastModified>2026-10-27T00:00:00Z</LastModified></Contents>", escape(prefix+name), len(f.data))
		}
		io.WriteString(w, "</ListBucketResult>")
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/")
	if s3 {
		p = strings.TrimPrefix(p, "bucket/")
	}
	parts := strings.SplitN(strings.TrimSuffix(p, "/"), "/", 2)
	root := parts[0]
	if r.Method == "PROPFIND" {
		if r.Header.Get("Depth") != "1" || len(parts) != 1 {
			http.Error(w, "depth", 400)
			return
		}
		w.WriteHeader(207)
		fmt.Fprintf(w, `<d:multistatus xmlns:d="DAV:"><d:response><d:href>/%s/</d:href></d:response>`, escape(root))
		for _, name := range entries(root) {
			f, _ := get(root, name)
			fmt.Fprintf(w, `<d:response><d:href>/%s/%s</d:href><d:propstat><d:prop><d:getcontentlength>%d</d:getcontentlength><d:getlastmodified>Tue, 27 Oct 2026 00:00:00 GMT</d:getlastmodified></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`, escape(root), escape(name), len(f.data))
		}
		io.WriteString(w, "</d:multistatus>")
		return
	}
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	f, ok := get(root, parts[1])
	if !ok {
		http.NotFound(w, r)
		return
	}
	if etag := r.Header.Get("If-Match"); etag != "" && etag != f.etag {
		w.WriteHeader(412)
		return
	}
	w.Header().Set("ETag", f.etag)
	http.ServeContent(w, r, path.Base(p), time.Unix(1793059200, 0), bytes.NewReader(f.data))
}
func main() {
	root := "/fixtures"
	items, _ := os.ReadDir(root)
	for _, item := range items {
		if item.IsDir() {
			continue
		}
		data, e := os.ReadFile(filepath.Join(root, item.Name()))
		if e != nil {
			log.Fatal("fixture read")
		}
		sum := sha256.Sum256(data)
		files[item.Name()] = fixture{data, "\"" + hex.EncodeToString(sum[:]) + "\""}
	}
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Local External Fixture"}, DNSNames: []string{"external-fixture"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(48 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, _ := x509.CreateCertificate(rand.Reader, template, template, public, private)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, _ := x509.MarshalPKCS8PrivateKey(private)
	cert, _ := tls.X509KeyPair(certPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	systemCA, _ := os.ReadFile("/etc/ssl/cert.pem")
	os.WriteFile("/out/fixture-ca.pem", append(systemCA, certPEM...), 0600)
	_, sshKey, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(sshKey)
	config := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
		if c.User() != "test" || string(p) != "test-only" {
			return nil, fmt.Errorf("denied")
		}
		return nil, nil
	}}
	config.AddHostKey(signer)
	cfg := sources.Config{}
	for _, kind := range []string{"webdav", "s3", "sftp"} {
		s := sources.Source{ID: "local-" + kind, Type: kind, Endpoint: "https://external-fixture:8443", Root: "music", SecretEnv: "NAVIDROME_SOURCE_LOCAL", ReadOnly: true, Enabled: true, AllowedNetworks: []string{"172.16.0.0/12"}}
		if kind == "s3" {
			s.Bucket = "bucket"
			s.Region = "us-east-1"
		}
		if kind == "sftp" {
			s.Endpoint = "sftp://external-fixture:2222"
			s.Root = "fixtures"
			s.HostKeySHA256 = ssh.FingerprintSHA256(signer.PublicKey())
		}
		cfg.Sources = append(cfg.Sources, s)
	}
	for _, count := range []int{100, 1000, 10000} {
		cfg.Sources = append(cfg.Sources, sources.Source{ID: fmt.Sprintf("scale-%d", count), Type: "s3", Endpoint: "https://external-fixture:8443", Root: fmt.Sprintf("scale%d", count), Bucket: "bucket", Region: "us-east-1", SecretEnv: "NAVIDROME_SOURCE_LOCAL", ReadOnly: true, Enabled: true, AllowedNetworks: []string{"172.16.0.0/12"}})
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile("/out/sources.json", data, 0600)
	go func() {
		listener, e := net.Listen("tcp", ":2222")
		if e != nil {
			log.Fatal("SSH listen")
		}
		for {
			c, e := listener.Accept()
			if e != nil {
				return
			}
			go func() {
				defer c.Close()
				conn, chans, requests, e := ssh.NewServerConn(c, config)
				if e != nil {
					return
				}
				defer conn.Close()
				go ssh.DiscardRequests(requests)
				for ch := range chans {
					if ch.ChannelType() != "session" {
						ch.Reject(ssh.UnknownChannelType, "session only")
						continue
					}
					channel, requests, e := ch.Accept()
					if e != nil {
						return
					}
					go func() {
						defer channel.Close()
						for req := range requests {
							if req.Type == "subsystem" && strings.Contains(string(req.Payload), "sftp") {
								req.Reply(true, nil)
								server, e := sftp.NewServer(channel, sftp.ReadOnly())
								if e == nil {
									server.Serve()
									server.Close()
								}
								return
							}
							req.Reply(false, nil)
						}
					}()
				}
			}()
		}
	}()
	listener, e := tls.Listen("tcp", ":8443", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	if e != nil {
		log.Fatal("TLS listen")
	}
	log.Print("Local read-only S3/WebDAV/SFTP fixture ready")
	(&http.Server{Handler: http.HandlerFunc(serve), ReadHeaderTimeout: 10 * time.Second}).Serve(listener)
}
