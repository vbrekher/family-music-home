// Package external registers operator-approved read-only remote music sources.
package external

import (
	"errors"
	"github.com/navidrome/navidrome/core/storage"
	"github.com/navidrome/navidrome/core/storage/remoteio"
	sources "github.com/navidrome/navidrome/integrations/external_sources"
	"io/fs"
	"net/url"
	"os"
	"strings"
)

type failed struct{}

func (failed) FS() (storage.MusicFS, error) {
	return nil, errors.New("external source unavailable or disabled")
}
func Sources() (sources.Config, error) {
	file := os.Getenv("ND_EXTERNALSOURCESFILE")
	if file == "" {
		return sources.Config{}, nil
	}
	data, e := os.ReadFile(file)
	if e != nil {
		return sources.Config{}, errors.New("external source configuration unavailable")
	}
	return sources.Parse(data)
}
func Resolve(raw string) (sources.Source, string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return sources.Source{}, "", fs.ErrInvalid
	}
	suffix := strings.TrimPrefix(u.Path, "/")
	if suffix != "" && (!fs.ValidPath(suffix) || strings.Contains(suffix, "\\")) {
		return sources.Source{}, "", fs.ErrInvalid
	}
	cfg, e := Sources()
	if e != nil {
		return sources.Source{}, "", e
	}
	for _, s := range cfg.Sources {
		if s.ID == u.Host && s.Type == u.Scheme && s.Enabled && enabled(s.ID) {
			return s, suffix, nil
		}
	}
	return sources.Source{}, "", errors.New("source is not enabled in administrator configuration")
}
func ValidateURI(raw string) error { _, _, e := Resolve(raw); return e }
func create(u url.URL) storage.Storage {
	s, suffix, e := Resolve(u.String())
	if e != nil {
		return failed{}
	}
	b, e := NewBackend(s, suffix)
	if e != nil {
		report(s.ID, e)
		return failed{}
	}
	return remoteio.New(monitored{s.ID, b})
}
func init() {
	for _, s := range []string{"s3", "sftp", "webdav"} {
		storage.Register(s, create)
	}
}
