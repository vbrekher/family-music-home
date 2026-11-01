// Package bunny provides read-only, on-demand access to Bunny Edge Storage.
// It never mirrors objects to disk. Callers own configuration and registration.
package bunny

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/navidrome/navidrome/adapters/gotaglib"
	"github.com/navidrome/navidrome/core/storage"
	"github.com/navidrome/navidrome/model/metadata"
)

// Config contains server-only secrets. Origin is a regional Storage API origin,
// not a public Pull Zone. Prefix is optional and must be an fs.ValidPath.
type Config struct {
	Origin, Zone, Prefix, AccessKey string
	Client                          *http.Client
}
type Storage struct{ f *MusicFS }
type MusicFS struct {
	base     *url.URL
	key      string
	client   *http.Client
	mu       sync.Mutex
	listings map[string]listing
}
type listing struct {
	entries []fs.DirEntry
	expires time.Time
}

var _ storage.Storage = (*Storage)(nil)
var _ storage.MusicFS = (*MusicFS)(nil)

func New(c Config) (*Storage, error) {
	u, e := url.Parse(c.Origin)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("bunny: origin must be an HTTPS origin without credentials, query or path")
	}
	if !fs.ValidPath(c.Zone) || strings.Contains(c.Zone, "/") || c.Zone == "." || c.AccessKey == "" {
		return nil, errors.New("bunny: valid zone and access key required")
	}
	if c.Prefix != "" && (!fs.ValidPath(c.Prefix) || c.Prefix == ".") {
		return nil, fs.ErrInvalid
	}
	u.Path = "/" + c.Zone + "/"
	if c.Prefix != "" {
		u.Path += c.Prefix + "/"
	}
	client := http.Client{Timeout: 30 * time.Second}
	if c.Client != nil {
		client = *c.Client
	}
	// Never forward the zone password through redirects (including same-host redirects).
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &Storage{&MusicFS{base: u, key: c.AccessKey, client: &client}}, nil
}
func (s *Storage) FS() (storage.MusicFS, error) { return s.f, nil }
func (f *MusicFS) request(ctx context.Context, name string, dir bool, ran string, etag ...string) (*http.Response, error) {
	u := *f.base
	if name != "." {
		u.Path += name
	}
	if dir && !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("AccessKey", f.key)
	req.Header.Set("Accept-Encoding", "identity")
	if ran != "" {
		req.Header.Set("Range", ran)
	}
	if len(etag) > 0 && etag[0] != "" {
		req.Header.Set("If-Match", etag[0])
	}
	for attempt := 0; attempt < 3; attempt++ {
		r, e := f.client.Do(req.Clone(ctx))
		if e != nil {
			return nil, errors.New("bunny: request failed")
		}
		if r.StatusCode != 429 && r.StatusCode < 500 {
			return r, nil
		}
		if attempt == 2 {
			return r, nil
		}
		r.Body.Close()
		timer := time.NewTimer(time.Duration(attempt+1) * 100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, errors.New("bunny: retry exhausted")
}
func status(code int) error {
	switch code {
	case 404:
		return fs.ErrNotExist
	case 401, 403:
		return fs.ErrPermission
	}
	return fmt.Errorf("bunny: unexpected HTTP status %d", code)
}

type object struct {
	ObjectName  string
	IsDirectory bool
	Length      int64
	LastChanged string
}
type info struct{ o object }

func (i info) Name() string { return i.o.ObjectName }
func (i info) Size() int64  { return i.o.Length }
func (i info) Mode() fs.FileMode {
	if i.IsDir() {
		return fs.ModeDir | 0555
	}
	return 0444
}
func (i info) ModTime() time.Time {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.9999999"} {
		if t, e := time.Parse(layout, i.o.LastChanged); e == nil {
			return t
		}
	}
	return time.Time{}
}
func (i info) IsDir() bool                { return i.o.IsDirectory }
func (i info) Sys() any                   { return nil }
func (i info) Type() fs.FileMode          { return i.Mode().Type() }
func (i info) Info() (fs.FileInfo, error) { return i, nil }
func (f *MusicFS) list(name string) ([]fs.DirEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if cached, ok := f.listings[name]; ok && time.Now().Before(cached.expires) {
		return cached.entries, nil
	}
	r, e := f.request(context.Background(), name, true, "")
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, status(r.StatusCode)
	}
	body, e := io.ReadAll(io.LimitReader(r.Body, (16<<20)+1))
	if e != nil {
		return nil, e
	}
	if len(body) > 16<<20 {
		return nil, errors.New("bunny: directory listing exceeds 16 MiB limit")
	}
	var objects []object
	if e = json.Unmarshal(body, &objects); e != nil {
		return nil, e
	}
	if objects == nil {
		return nil, errors.New("bunny: directory listing must be an array")
	}
	out := make([]fs.DirEntry, 0, len(objects))
	for _, o := range objects {
		if !fs.ValidPath(o.ObjectName) || o.ObjectName == "." || strings.Contains(o.ObjectName, "/") || o.Length < 0 {
			return nil, errors.New("bunny: invalid directory entry")
		}
		out = append(out, info{o})
	}
	if len(out) <= 20000 {
		total := len(out)
		for _, v := range f.listings {
			total += len(v.entries)
		}
		if len(f.listings) >= 64 || total > 20000 {
			f.listings = make(map[string]listing)
		}
		if f.listings == nil {
			f.listings = make(map[string]listing)
		}
		f.listings[name] = listing{out, time.Now().Add(2 * time.Second)}
	}
	return out, nil
}
func (f *MusicFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	if name == "." {
		if _, e := f.list(name); e != nil {
			return nil, e
		}
		return info{object{ObjectName: ".", IsDirectory: true}}, nil
	}
	entries, e := f.list(path.Dir(name))
	if e != nil {
		return nil, e
	}
	for _, ent := range entries {
		if ent.Name() == path.Base(name) {
			return ent.Info()
		}
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}
func (f *MusicFS) Open(name string) (fs.File, error) {
	i, e := f.Stat(name)
	if e != nil {
		return nil, e
	}
	h := &file{fs: f, name: name, info: i}
	return h, nil
}
func (f *MusicFS) ReadTags(names ...string) (map[string]metadata.Info, error) {
	for _, n := range names {
		if !fs.ValidPath(n) {
			return nil, fs.ErrInvalid
		}
	}
	tags, e := gotaglib.NewExtractor(f).Parse(names...)
	if e != nil {
		return nil, e
	}
	// The upstream extractor omits failures; do not claim a successful partial batch.
	if len(tags) != len(names) {
		return nil, errors.New("bunny: metadata extraction failed for one or more files")
	}
	for name, m := range tags {
		i, e := f.Stat(name)
		if e != nil {
			return nil, e
		}
		m.FileInfo = tagInfo{i}
		tags[name] = m
	}
	return tags, nil
}

type tagInfo struct{ fs.FileInfo }

func (tagInfo) BirthTime() time.Time { return time.Time{} }

type file struct {
	fs             *MusicFS
	name           string
	info           fs.FileInfo
	pos            int64
	entries        []fs.DirEntry
	block          []byte
	blockStart     int64
	etag           string
	listed, closed bool
}

func (h *file) Stat() (fs.FileInfo, error) {
	if h.closed {
		return nil, fs.ErrClosed
	}
	return h.info, nil
}
func (h *file) Close() error { h.closed = true; return nil }
func (h *file) Seek(offset int64, whence int) (int64, error) {
	if h.closed {
		return 0, fs.ErrClosed
	}
	if h.info.IsDir() {
		return 0, fs.ErrInvalid
	}
	p := offset
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		p += h.pos
	case io.SeekEnd:
		p += h.info.Size()
	default:
		return 0, fs.ErrInvalid
	}
	if p < 0 {
		return 0, fs.ErrInvalid
	}
	h.pos = p
	return p, nil
}
func (h *file) Read(p []byte) (int, error) {
	if h.closed {
		return 0, fs.ErrClosed
	}
	if h.info.IsDir() {
		return 0, fs.ErrInvalid
	}
	if len(p) == 0 {
		return 0, nil
	}
	if h.pos >= h.info.Size() {
		return 0, io.EOF
	}
	// One 256 KiB block per open file; metadata seek patterns reuse this block.
	if len(h.block) > 0 && h.pos >= h.blockStart && h.pos < h.blockStart+int64(len(h.block)) {
		n := copy(p, h.block[h.pos-h.blockStart:])
		h.pos += int64(n)
		return n, nil
	}
	start := h.pos / (256 << 10) * (256 << 10)
	end := start + (256 << 10) - 1
	if end >= h.info.Size() {
		end = h.info.Size() - 1
	}
	r, e := h.fs.request(context.Background(), h.name, false, fmt.Sprintf("bytes=%d-%d", start, end), h.etag)
	if e != nil {
		return 0, e
	}
	defer r.Body.Close()
	if r.StatusCode != 206 {
		return 0, status(r.StatusCode)
	}
	expected := "bytes " + strconv.FormatInt(start, 10) + "-" + strconv.FormatInt(end, 10) + "/" + strconv.FormatInt(h.info.Size(), 10)
	if r.Header.Get("Content-Range") != expected || (r.Header.Get("Content-Encoding") != "" && r.Header.Get("Content-Encoding") != "identity") {
		return 0, errors.New("bunny: invalid range response")
	}
	etag := r.Header.Get("ETag")
	if h.etag != "" && etag != h.etag {
		return 0, errors.New("bunny: object changed during read")
	}
	if etag == "" || strings.HasPrefix(etag, "W/") {
		return 0, errors.New("bunny: strong ETag required for consistent random reads")
	}
	block := make([]byte, end-start+1)
	if _, e = io.ReadFull(r.Body, block); e != nil {
		return 0, e
	}
	var extra [1]byte
	if n, e := r.Body.Read(extra[:]); n != 0 || e != io.EOF {
		return 0, errors.New("bunny: oversized range response")
	}
	h.block = block
	h.blockStart = start
	h.etag = etag
	n := copy(p, h.block[h.pos-start:])
	h.pos += int64(n)
	return n, nil
}
func (h *file) ReadDir(n int) ([]fs.DirEntry, error) {
	if h.closed {
		return nil, fs.ErrClosed
	}
	if !h.info.IsDir() {
		return nil, fs.ErrInvalid
	}
	if !h.listed {
		v, e := h.fs.list(h.name)
		if e != nil {
			return nil, e
		}
		h.entries = v
		h.listed = true
	}
	if h.pos >= int64(len(h.entries)) {
		if n > 0 {
			return nil, io.EOF
		}
		return []fs.DirEntry{}, nil
	}
	end := len(h.entries)
	if n > 0 && int(h.pos)+n < end {
		end = int(h.pos) + n
	}
	v := append([]fs.DirEntry(nil), h.entries[h.pos:end]...)
	h.pos = int64(end)
	return v, nil
}
