package external

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"github.com/navidrome/navidrome/core/storage/remoteio"
	sources "github.com/navidrome/navidrome/integrations/external_sources"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testSource(t *testing.T, kind, endpoint string) sources.Source {
	t.Helper()
	t.Setenv("NAVIDROME_SOURCE_TEST", `{"username":"test","password":"test-only","access_key":"test","secret_key":"test-only"}`)
	return sources.Source{ID: "test", Type: kind, Endpoint: endpoint, Root: "music", SecretEnv: "NAVIDROME_SOURCE_TEST", ReadOnly: true, Enabled: true, AllowedNetworks: []string{"127.0.0.0/8"}}
}
func exercise(t *testing.T, b remoteio.Backend, payload []byte) {
	t.Helper()
	store := remoteio.New(b)
	f, _ := store.FS()
	entries, e := fs.ReadDir(f, ".")
	if e != nil || len(entries) != 1 || entries[0].Name() != "tone.flac" {
		t.Fatalf("listing: %v %v", entries, e)
	}
	file, e := f.Open("tone.flac")
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	raw, e := io.ReadAll(file)
	if e != nil || !bytes.Equal(raw, payload) {
		t.Fatalf("original read: %v len=%d", e, len(raw))
	}
	seeker := file.(io.ReadSeeker)
	_, e = seeker.Seek(12345, io.SeekStart)
	if e != nil {
		t.Fatal(e)
	}
	got := make([]byte, 345)
	_, e = io.ReadFull(seeker, got)
	if e != nil || !bytes.Equal(got, payload[12345:12690]) {
		t.Fatal("seek differs", e)
	}
	if _, e = f.Open("../escape"); e == nil {
		t.Fatal("traversal accepted")
	}
}
func TestHTTPSWebDAVReadOnlyRangeAndConsistency(t *testing.T) {
	payload := bytes.Repeat([]byte("flac-fixture"), 100000)
	var wrong atomic.Bool
	var count atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		u, p, ok := r.BasicAuth()
		if !ok || u != "test" || p != "test-only" {
			t.Error("missing auth")
			w.WriteHeader(403)
			return
		}
		switch r.Method {
		case "PROPFIND":
			if r.Header.Get("Depth") != "1" {
				t.Error("unbounded depth")
			}
			w.WriteHeader(207)
			fmt.Fprintf(w, `<d:multistatus xmlns:d="DAV:"><d:response><d:href>/music/</d:href></d:response><d:response><d:href>/music/tone.flac</d:href><d:propstat><d:prop><d:getcontentlength>%d</d:getcontentlength><d:getlastmodified>Mon, 26 Oct 2026 12:00:00 GMT</d:getlastmodified></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`, len(payload))
		case "GET", "HEAD":
			w.Header().Set("ETag", `"v1"`)
			if wrong.Load() && r.Method == "GET" {
				w.WriteHeader(200)
				return
			}
			if match := r.Header.Get("If-Match"); match != "" && match != `"v1"` {
				t.Error("wrong version pin")
			}
			http.ServeContent(w, r, "tone.flac", zeroTime, bytes.NewReader(payload))
		default:
			t.Errorf("write or unsupported request %s", r.Method)
			w.WriteHeader(405)
		}
	}))
	defer server.Close()
	s := testSource(t, "webdav", server.URL)
	backend, e := NewBackend(s, "")
	if e != nil {
		t.Fatal(e)
	}
	b := backend.(*httpBackend)
	b.client = server.Client()
	exercise(t, b, payload)
	wrong.Store(true)
	fileStore := remoteio.New(b)
	f, _ := fileStore.FS()
	file, e := f.Open("tone.flac")
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	if _, e = file.Read(make([]byte, 10)); e == nil {
		t.Fatal("HTTP 200 accepted for range")
	}
	if count.Load() < 3 {
		t.Fatal("no real HTTP traffic")
	}
}
func TestS3PaginationSigningAndReadOnlyRanges(t *testing.T) {
	payload := bytes.Repeat([]byte("s3-fixture"), 100000)
	var pages atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=test/") || r.Header.Get("X-Amz-Content-Sha256") != hash("") {
			t.Error("SigV4 missing")
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			t.Error("write requested")
			w.WriteHeader(405)
			return
		}
		if r.URL.Query().Get("list-type") == "2" {
			pages.Add(1)
			if r.URL.Query().Get("prefix") != "music/" || r.URL.Query().Get("delimiter") != "/" {
				t.Error("prefix jail missing")
			}
			if r.URL.Query().Get("continuation-token") == "" {
				fmt.Fprint(w, `<ListBucketResult><IsTruncated>true</IsTruncated><NextContinuationToken>second</NextContinuationToken></ListBucketResult>`)
			} else {
				fmt.Fprintf(w, `<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>music/tone.flac</Key><Size>%d</Size></Contents></ListBucketResult>`, len(payload))
			}
			return
		}
		if r.URL.Path != "/bucket/music/tone.flac" {
			t.Error("object escaped bucket/prefix")
		}
		w.Header().Set("ETag", `"s3-version-1"`)
		http.ServeContent(w, r, "tone.flac", zeroTime, bytes.NewReader(payload))
	}))
	defer server.Close()
	s := testSource(t, "s3", server.URL)
	s.Bucket = "bucket"
	s.Region = "us-east-1"
	backend, e := NewBackend(s, "")
	if e != nil {
		t.Fatal(e)
	}
	b := backend.(*httpBackend)
	b.client = server.Client()
	exercise(t, b, payload)
	if pages.Load() != 2 {
		t.Fatalf("pagination count %d", pages.Load())
	}
}
func TestSFTPRealSSHReadOnlyAndHostPin(t *testing.T) {
	root := t.TempDir()
	payload := bytes.Repeat([]byte("sftp-fixture"), 100000)
	if e := os.WriteFile(filepath.Join(root, "tone.flac"), payload, 0600); e != nil {
		t.Fatal(e)
	}
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(private)
	config := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
		if c.User() != "test" || string(p) != "test-only" {
			return nil, fmt.Errorf("denied")
		}
		return nil, nil
	}}
	config.AddHostKey(signer)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	go func() {
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
	s := testSource(t, "sftp", "sftp://"+listener.Addr().String())
	s.Root = strings.TrimPrefix(root, "/")
	s.HostKeySHA256 = ssh.FingerprintSHA256(signer.PublicKey())
	b, e := NewBackend(s, "")
	if e != nil {
		t.Fatal(e)
	}
	exercise(t, b, payload)
	s.HostKeySHA256 = "SHA256:" + strings.Repeat("A", 43)
	bad, e := NewBackend(s, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = bad.List(context.Background(), "."); e == nil {
		t.Fatal("incorrect host key accepted")
	}
}
func TestPrivateOriginsRequireExplicitApproval(t *testing.T) {
	s := testSource(t, "webdav", "https://127.0.0.1:1234")
	s.AllowedNetworks = nil
	if _, e := NewBackend(s, ""); e == nil {
		t.Fatal("unapproved loopback accepted")
	}
}
func TestDAVRejectsPartialAndEscapedListing(t *testing.T) {
	for _, body := range []string{`<multistatus xmlns="DAV:">`, `<multistatus xmlns="DAV:"><response><href>/music/</href></response><response><href>https://evil.invalid/secret</href></response></multistatus>`} {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(207); io.WriteString(w, body) }))
		s := testSource(t, "webdav", server.URL)
		b, _ := NewBackend(s, "")
		b.(*httpBackend).client = server.Client()
		if _, e := b.List(context.Background(), "."); e == nil {
			t.Fatal("bad listing accepted")
		}
		server.Close()
	}
}

var zeroTime = time.Time{}

func TestS3VirtualDirectoriesCanBeWalked(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("list-type") == "2" {
			if r.URL.Query().Get("prefix") == "music/" {
				io.WriteString(w, `<ListBucketResult><IsTruncated>false</IsTruncated><CommonPrefixes><Prefix>music/sub/</Prefix></CommonPrefixes></ListBucketResult>`)
			} else {
				io.WriteString(w, `<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>music/sub/tone.flac</Key><Size>10</Size></Contents></ListBucketResult>`)
			}
			return
		}
		if r.URL.Path != "/bucket/music/sub/tone.flac" {
			t.Error("directory incorrectly requested as object")
		}
		w.Header().Set("ETag", `"nested"`)
		http.ServeContent(w, r, "tone.flac", zeroTime, strings.NewReader("0123456789"))
	}))
	defer server.Close()
	s := testSource(t, "s3", server.URL)
	s.Bucket = "bucket"
	s.Region = "us-east-1"
	backend, e := NewBackend(s, "")
	if e != nil {
		t.Fatal(e)
	}
	backend.(*httpBackend).client = server.Client()
	store := remoteio.New(backend)
	f, _ := store.FS()
	names := []string{}
	e = fs.WalkDir(f, ".", func(name string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		names = append(names, name)
		return nil
	})
	if e != nil || len(names) != 3 {
		t.Fatalf("nested walk failed %v %v", names, e)
	}
	data, e := fs.ReadFile(f, "sub/tone.flac")
	if e != nil || string(data) != "0123456789" {
		t.Fatal("nested read failed", e)
	}
}

func TestWebDAVNestedCollectionsAndRedirectRefusal(t *testing.T) {
	var redirected bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirected" {
			redirected = true
			w.WriteHeader(207)
			return
		}
		if r.URL.Path == "/music/redirect" {
			http.Redirect(w, r, "/redirected", 302)
			return
		}
		if r.Method == "PROPFIND" {
			w.WriteHeader(207)
			base := strings.TrimSuffix(r.URL.Path, "/")
			fmt.Fprintf(w, `<d:multistatus xmlns:d="DAV:"><d:response><d:href>%s/</d:href></d:response>`, base)
			if base == "/music" {
				fmt.Fprintf(w, `<d:response><d:href>/music/Album/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`)
			} else {
				fmt.Fprintf(w, `<d:response><d:href>/music/Album/tone.flac</d:href><d:propstat><d:prop><d:getcontentlength>10</d:getcontentlength></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`)
			}
			io.WriteString(w, `</d:multistatus>`)
			return
		}
		w.Header().Set("ETag", `"nested-dav"`)
		http.ServeContent(w, r, "tone.flac", zeroTime, strings.NewReader("0123456789"))
	}))
	defer server.Close()
	s := testSource(t, "webdav", server.URL)
	backend, e := NewBackend(s, "")
	if e != nil {
		t.Fatal(e)
	}
	backend.(*httpBackend).client.Transport = server.Client().Transport
	store := remoteio.New(backend)
	f, _ := store.FS()
	paths := []string{}
	e = fs.WalkDir(f, ".", func(n string, _ fs.DirEntry, e error) error { paths = append(paths, n); return e })
	if e != nil || len(paths) != 3 {
		t.Fatalf("WebDAV nested walk failed %v %v", paths, e)
	}
	data, e := fs.ReadFile(f, "Album/tone.flac")
	if e != nil || string(data) != "0123456789" {
		t.Fatal("nested DAV read failed", e)
	}
	if _, e = backend.List(context.Background(), "redirect"); e == nil || redirected {
		t.Fatal("redirect followed or accepted")
	}
}
