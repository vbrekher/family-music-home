package bunny

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T, bad bool) (*MusicFS, *httptest.Server) {
	t.Helper()
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("AccessKey") != "secret" {
			t.Error("key missing")
		}
		if r.Method != "GET" {
			t.Error("mutation")
		}
		if r.URL.Path == "/zone/" {
			io.WriteString(w, `[{"ObjectName":"a #%.mp3","Length":10}]`)
			return
		}
		if r.URL.Path != "/zone/a #%.mp3" {
			w.WriteHeader(404)
			return
		}
		var a, b int
		if _, e := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &a, &b); e != nil {
			t.Fatal(e)
		}
		if bad {
			io.WriteString(w, "0123456789")
			return
		}
		w.Header().Set("ETag", `"fixture-v1"`)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/10", a, b))
		w.WriteHeader(206)
		io.WriteString(w, "0123456789"[a:b+1])
	}))
	store, e := New(Config{Origin: s.URL, Zone: "zone", AccessKey: "secret", Client: s.Client()})
	if e != nil {
		t.Fatal(e)
	}
	return store.f, s
}
func TestReadSeek(t *testing.T) {
	f, s := fixture(t, false)
	defer s.Close()
	h, e := f.Open("a #%.mp3")
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	r := h.(io.ReadSeeker)
	p := make([]byte, 3)
	if _, e = r.Read(p); e != nil || string(p) != "012" {
		t.Fatalf("%q %v", p, e)
	}
	r.Seek(-3, io.SeekEnd)
	if _, e = r.Read(p); e != nil || string(p) != "789" {
		t.Fatalf("%q %v", p, e)
	}
	if _, e = r.Read(p); e != io.EOF {
		t.Fatal(e)
	}
	if _, e = r.Seek(-11, io.SeekEnd); e == nil {
		t.Fatal("negative seek accepted")
	}
	h.Close()
	if _, e = r.Read(p); e != fs.ErrClosed {
		t.Fatal(e)
	}
}
func TestRejectIgnoredRange(t *testing.T) {
	f, s := fixture(t, true)
	defer s.Close()
	h, e := f.Open("a #%.mp3")
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	if _, e = h.Read(make([]byte, 3)); e == nil {
		t.Fatal("accepted 200 for range")
	}
}
func TestReadDirPaths(t *testing.T) {
	f, s := fixture(t, false)
	defer s.Close()
	h, e := f.Open(".")
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	d := h.(fs.ReadDirFile)
	v, e := d.ReadDir(1)
	if e != nil || len(v) != 1 {
		t.Fatalf("%v %v", v, e)
	}
	if _, e = d.ReadDir(1); e != io.EOF {
		t.Fatal(e)
	}
	for _, p := range []string{"", "../x", "/root", "a/../b", "a//b"} {
		if _, e = f.Open(p); e == nil {
			t.Fatal(p)
		}
	}
}
func TestValidateConfig(t *testing.T) {
	for _, origin := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com/path", "https://example.com?key=x"} {
		if _, e := New(Config{Origin: origin, Zone: "zone", AccessKey: "x"}); e == nil {
			t.Fatal(origin)
		}
	}
}
func TestRedirectAndListingFailure(t *testing.T) {
	for _, code := range []int{302, 503} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "https://example.com")
				w.WriteHeader(code)
			}))
			defer s.Close()
			store, e := New(Config{Origin: s.URL, Zone: "zone", AccessKey: "x", Client: s.Client()})
			if e != nil {
				t.Fatal(e)
			}
			_, e = store.f.Open(".")
			if e == nil {
				t.Fatal("failed list interpreted as empty")
			}
		})
	}
}

func TestTagsFromRemoteFixture(t *testing.T) {
	data, e := os.ReadFile("../../../tests/fixtures/test.mp3")
	if e != nil {
		t.Fatal(e)
	}
	gets := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			fmt.Fprintf(w, `[{"ObjectName":"test.mp3","Length":%d}]`, len(data))
			return
		}
		gets++
		w.Header().Set("ETag", `"fixture"`)
		http.ServeContent(w, r, "test.mp3", info{}.ModTime(), bytes.NewReader(data))
	}))
	defer srv.Close()
	store, e := New(Config{Origin: srv.URL, Zone: "zone", AccessKey: "x", Client: srv.Client()})
	if e != nil {
		t.Fatal(e)
	}
	tags, e := store.f.ReadTags("test.mp3")
	if e != nil {
		t.Fatal(e)
	}
	m, ok := tags["test.mp3"]
	if !ok || m.AudioProperties.Duration <= 0 || m.FileInfo == nil {
		t.Fatalf("missing real metadata: %#v", m)
	}
	if gets == 0 {
		t.Fatal("metadata did not read remote data")
	}
}
func TestBlockCacheAndMutation(t *testing.T) {
	data := bytes.Repeat([]byte("a"), 300<<10)
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			fmt.Fprintf(w, `[{"ObjectName":"test.mp3","Length":%d}]`, len(data))
			return
		}
		calls++
		tag := `"one"`
		if calls > 1 {
			if r.Header.Get("If-Match") != tag {
				t.Error("missing If-Match")
			}
			tag = `"two"`
		}
		w.Header().Set("ETag", tag)
		http.ServeContent(w, r, "test.mp3", info{}.ModTime(), bytes.NewReader(data))
	}))
	defer srv.Close()
	store, e := New(Config{Origin: srv.URL, Zone: "zone", AccessKey: "x", Client: srv.Client()})
	if e != nil {
		t.Fatal(e)
	}
	h, e := store.f.Open("test.mp3")
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	p := make([]byte, 1)
	if _, e = h.Read(p); e != nil {
		t.Fatal(e)
	}
	if _, e = h.Read(p); e != nil {
		t.Fatal(e)
	}
	if calls != 1 {
		t.Fatal("block was not cached")
	}
	h.(io.Seeker).Seek(270<<10, io.SeekStart)
	if _, e = h.Read(p); e == nil {
		t.Fatal("changed ETag accepted")
	}
}

func TestRejectInvalidRevisionAndRange(t *testing.T) {
	for _, tc := range []struct{ tag, contentRange string }{{"", "bytes 0-9/10"}, {`W/"v1"`, "bytes 0-9/10"}, {`"v1"`, "bytes 1-9/10"}} {
		t.Run(tc.tag+tc.contentRange, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/") {
					io.WriteString(w, `[{"ObjectName":"f.mp3","Length":10}]`)
					return
				}
				w.Header().Set("ETag", tc.tag)
				w.Header().Set("Content-Range", tc.contentRange)
				w.WriteHeader(206)
				io.WriteString(w, "0123456789")
			}))
			defer srv.Close()
			store, e := New(Config{Origin: srv.URL, Zone: "zone", AccessKey: "x", Client: srv.Client()})
			if e != nil {
				t.Fatal(e)
			}
			h, e := store.f.Open("f.mp3")
			if e != nil {
				t.Fatal(e)
			}
			defer h.Close()
			if _, e = h.Read(make([]byte, 2)); e == nil {
				t.Fatal("unsafe response accepted")
			}
		})
	}
}

// Explicitly opt-in; never runs against live storage during normal unit tests.
func TestLiveStorage(t *testing.T) {
	secretFile := os.Getenv("BUNNY_TEST_CREDENTIALS_FILE")
	if secretFile == "" {
		t.Skip("live credentials not configured")
	}
	data, e := os.ReadFile(secretFile)
	if e != nil {
		t.Fatal("cannot read live credential file")
	}
	cfg := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if ok {
			cfg[k] = strings.Trim(strings.TrimSpace(v), "\"'")
		}
	}
	store, e := New(Config{Origin: "https://storage.bunnycdn.com", Zone: cfg["BUNNY_STORAGE_ZONE"], AccessKey: cfg["BUNNY_STORAGE_PASSWORD"]})
	if e != nil {
		t.Fatal("invalid live storage config")
	}
	names := []string{"music/test/Музика 🐰/Family test.mp3", "music/test/Музика 🐰/Hi Res 24bit 96kHz.flac"}
	tags, e := store.f.ReadTags(names...)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range names {
		m, ok := tags[name]
		if !ok || m.AudioProperties.Duration <= 0 {
			t.Fatal("missing real metadata")
		}
		h, e := store.f.Open(name)
		if e != nil {
			t.Fatal(e)
		}
		r := h.(io.ReadSeeker)
		if _, e = r.Seek(-100, io.SeekEnd); e != nil {
			t.Fatal(e)
		}
		if _, e = io.ReadFull(r, make([]byte, 100)); e != nil {
			t.Fatal(e)
		}
		h.Close()
	}
}

func TestCatalogRequestCounts(t *testing.T) {
	for _, count := range []int{100, 1000, 10000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			objects := make([]object, count)
			for i := range objects {
				objects[i] = object{ObjectName: fmt.Sprintf("track-%05d.mp3", i), Length: 10}
			}
			var requests atomic.Int32
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); json.NewEncoder(w).Encode(objects) }))
			defer srv.Close()
			store, e := New(Config{Origin: srv.URL, Zone: "zone", AccessKey: "x", Client: srv.Client()})
			if e != nil {
				t.Fatal(e)
			}
			started := time.Now()
			h, e := store.f.Open(".")
			if e != nil {
				t.Fatal(e)
			}
			entries, e := h.(fs.ReadDirFile).ReadDir(-1)
			if e != nil {
				t.Fatal(e)
			}
			for _, entry := range entries {
				if _, e = store.f.Stat(entry.Name()); e != nil {
					t.Fatal(e)
				}
			}
			t.Logf("entries=%d requests=%d elapsed=%s", count, requests.Load(), time.Since(started))
			// Slow/contended local Docker runs may legitimately cross the two-second TTL.
			maxRequests := int32(time.Since(started)/(2*time.Second)) + 2
			if requests.Load() > maxRequests {
				t.Fatal("listing cache did not bound requests within its TTL")
			}
		})
	}
}
func TestTransientRetryAndFailureNotCached(t *testing.T) {
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 3 {
			w.WriteHeader(503)
			return
		}
		io.WriteString(w, "[]")
	}))
	defer srv.Close()
	store, e := New(Config{Origin: srv.URL, Zone: "zone", AccessKey: "x", Client: srv.Client()})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = store.f.Open("."); e == nil || calls != 3 {
		t.Fatalf("retry failure %v calls %d", e, calls)
	}
	if _, e = store.f.Open("."); e != nil || calls != 4 {
		t.Fatalf("failure cached %v calls %d", e, calls)
	}
}

func TestRejectPartialAndNullListing(t *testing.T) {
	for _, body := range []string{"null", `[{"ObjectName":"f.mp3","Length":10}`, `[] trailing`} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) }))
			defer srv.Close()
			store, e := New(Config{Origin: srv.URL, Zone: "zone", AccessKey: "x", Client: srv.Client()})
			if e != nil {
				t.Fatal(e)
			}
			if _, e = store.f.Open("."); e == nil {
				t.Fatal("invalid listing accepted as empty/complete")
			}
		})
	}
}
