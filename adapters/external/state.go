package external

import (
	"context"
	"github.com/navidrome/navidrome/core/storage/remoteio"
	"github.com/navidrome/navidrome/model"
	"io/fs"
	"net/http"
	"path"
	"sync"
)

var state struct {
	sync.RWMutex
	enabled  func(string) bool
	report   func(string, string)
	statuses map[string]string
}

// BindDataStore must run in both the HTTP server and the standalone scanner process.
// UI toggles are persisted in SQLite, not inherited Go process memory.
func BindDataStore(ds model.DataStore) {
	SetStatusReporter(func(id, status string) { _ = ds.Property(context.Background()).Put("external.status."+id, status) })
	SetEnabledCheck(func(id string) bool {
		value, err := ds.Property(context.Background()).DefaultGet("external.disabled."+id, "false")
		return err == nil && value != "true"
	})
}

func SetEnabledCheck(check func(string) bool) {
	state.Lock()
	defer state.Unlock()
	state.enabled = check
}
func SetStatusReporter(report func(string, string)) {
	state.Lock()
	defer state.Unlock()
	state.report = report
	state.statuses = map[string]string{}
}
func enabled(id string) bool {
	state.RLock()
	check := state.enabled
	state.RUnlock()
	return check == nil || check(id)
}
func report(id string, err error) {
	s := "Online"
	if err != nil {
		s = "Offline"
	}
	state.Lock()
	if state.statuses == nil {
		state.statuses = map[string]string{}
	}
	if state.statuses[id] == s {
		state.Unlock()
		return
	}
	state.statuses[id] = s
	fn := state.report
	state.Unlock()
	if fn != nil {
		fn(id, s)
	}
}

type monitored struct {
	id      string
	backend remoteio.Backend
}

func (b monitored) List(c context.Context, n string) ([]fs.DirEntry, error) {
	if !enabled(b.id) {
		return nil, fs.ErrPermission
	}
	out, e := b.backend.List(c, n)
	report(b.id, e)
	return out, e
}
func (b monitored) Range(c context.Context, n, r, eTag string) (*http.Response, error) {
	if !enabled(b.id) {
		return nil, fs.ErrPermission
	}
	rSP, e := b.backend.Range(c, n, r, eTag)
	if e == nil && rSP.StatusCode != 206 {
		report(b.id, fs.ErrInvalid)
	} else {
		report(b.id, e)
	}
	return rSP, e
}

func (b monitored) Stat(c context.Context, n string) (fs.FileInfo, error) {
	if !enabled(b.id) {
		return nil, fs.ErrPermission
	}
	if s, ok := b.backend.(interface {
		Stat(context.Context, string) (fs.FileInfo, error)
	}); ok {
		v, e := s.Stat(c, n)
		report(b.id, e)
		return v, e
	}
	entries, e := b.backend.List(c, path.Dir(n))
	report(b.id, e)
	if e != nil {
		return nil, e
	}
	for _, v := range entries {
		if v.Name() == path.Base(n) {
			return v.Info()
		}
	}
	return nil, fs.ErrNotExist
}
