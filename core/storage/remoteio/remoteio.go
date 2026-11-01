// Package remoteio provides bounded, read-only seekable music files for remote providers.
package remoteio

import (
	"context"
	"errors"
	"fmt"
	"github.com/navidrome/navidrome/adapters/gotaglib"
	"github.com/navidrome/navidrome/core/storage"
	"github.com/navidrome/navidrome/model/metadata"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Backend interface {
	List(context.Context, string) ([]fs.DirEntry, error)
	Range(context.Context, string, string, string) (*http.Response, error)
}
type Storage struct{ F *MusicFS }

func (s *Storage) FS() (storage.MusicFS, error) { return s.F, nil }

type MusicFS struct {
	Backend Backend
	mu      sync.Mutex
	cache   map[string]cachedListing
}
type cachedListing struct {
	entries []fs.DirEntry
	index   map[string]fs.DirEntry
	expires time.Time
}

func New(b Backend) *Storage { return &Storage{&MusicFS{Backend: b}} }
func (f *MusicFS) list(name string) ([]fs.DirEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if v, ok := f.cache[name]; ok && time.Now().Before(v.expires) {
		return v.entries, nil
	}
	entries, e := f.Backend.List(context.Background(), name)
	if e != nil {
		return nil, e
	}
	if entries == nil {
		return nil, errors.New("remote: incomplete listing")
	}
	seen := map[string]bool{}
	for _, v := range entries {
		if !fs.ValidPath(v.Name()) || strings.ContainsAny(v.Name(), "/\\") || v.Name() == "." || seen[v.Name()] {
			return nil, fs.ErrInvalid
		}
		seen[v.Name()] = true
	}
	total := len(entries)
	for _, item := range f.cache {
		total += len(item.entries)
	}
	if f.cache == nil || len(f.cache) >= 64 || total > 20000 {
		f.cache = map[string]cachedListing{}
	}
	if len(entries) <= 10000 {
		index := make(map[string]fs.DirEntry, len(entries))
		for _, entry := range entries {
			index[entry.Name()] = entry
		}
		f.cache[name] = cachedListing{entries: entries, index: index, expires: time.Now().Add(2 * time.Second)}
	}
	return entries, nil
}
func (f *MusicFS) request(c context.Context, n string, _ bool, r string, etags ...string) (*http.Response, error) {
	etag := ""
	if len(etags) > 0 {
		etag = etags[0]
	}
	return f.Backend.Range(c, n, r, etag)
}
func status(code int) error { return fmt.Errorf("remote: HTTP status %d", code) }

type Entry struct {
	Version   string
	Filename  string
	Length    int64
	Directory bool
	Modified  time.Time
}

func (i Entry) ETag() string { return i.Version }
func (i Entry) Name() string { return i.Filename }
func (i Entry) Size() int64  { return i.Length }
func (i Entry) IsDir() bool  { return i.Directory }
func (i Entry) Mode() fs.FileMode {
	if i.Directory {
		return fs.ModeDir | 0555
	}
	return 0444
}
func (i Entry) ModTime() time.Time         { return i.Modified }
func (i Entry) Sys() any                   { return nil }
func (i Entry) Type() fs.FileMode          { return i.Mode().Type() }
func (i Entry) Info() (fs.FileInfo, error) { return i, nil }
func (f *MusicFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	if name == "." {
		if _, e := f.list(name); e != nil {
			return nil, e
		}
		return Entry{Filename: ".", Directory: true}, nil
	}
	entries, e := f.list(path.Dir(name))
	if e != nil {
		return nil, e
	}
	f.mu.Lock()
	cached, ok := f.cache[path.Dir(name)]
	var ent fs.DirEntry
	if ok {
		ent = cached.index[path.Base(name)]
	}
	f.mu.Unlock()
	if !ok {
		for _, candidate := range entries {
			if candidate.Name() == path.Base(name) {
				ent = candidate
				break
			}
		}
	}
	if ent != nil {
		if !ent.IsDir() {
			if statter, ok := f.Backend.(interface {
				Stat(context.Context, string) (fs.FileInfo, error)
			}); ok {
				return statter.Stat(context.Background(), name)
			}
		}
		return ent.Info()
	}

	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}
func (f *MusicFS) Open(name string) (fs.File, error) {
	i, e := f.Stat(name)
	if e != nil {
		return nil, e
	}
	h := &file{fs: f, name: name, info: i}
	if v, ok := i.(interface{ ETag() string }); ok {
		h.etag = v.ETag()
	}
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
		return nil, errors.New("remote: metadata extraction failed for one or more files")
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
func (h *file) Close() error { h.closed = true; h.block = nil; h.entries = nil; return nil }
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
		return 0, errors.New("remote: invalid range response")
	}
	etag := r.Header.Get("ETag")
	if h.etag != "" && etag != h.etag {
		return 0, errors.New("remote: object changed during read")
	}
	if etag == "" || strings.HasPrefix(etag, "W/") {
		return 0, errors.New("remote: strong ETag required for consistent random reads")
	}
	block := make([]byte, end-start+1)
	if _, e = io.ReadFull(r.Body, block); e != nil {
		return 0, e
	}
	var extra [1]byte
	if n, e := r.Body.Read(extra[:]); n != 0 || e != io.EOF {
		return 0, errors.New("remote: oversized range response")
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
