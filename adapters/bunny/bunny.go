// Package bunny registers private, read-only Bunny Storage libraries.
package bunny

import (
	"errors"
	"io/fs"
	"net/url"
	"os"
	"strings"

	"github.com/navidrome/navidrome/core/storage"
	bunnystore "github.com/navidrome/navidrome/core/storage/bunny"
)

// ValidateURI prevents library paths from selecting other zones or carrying credentials.
func ValidateURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("bunny: invalid library URI")
	}
	return validate(*u)
}
func validate(u url.URL) error {
	zone := os.Getenv("BUNNY_STORAGE_ZONE")
	if u.Scheme != "bunny" || zone == "" || u.Host != zone || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return errors.New("bunny: library must reference the configured private storage zone")
	}
	prefix := strings.TrimPrefix(u.Path, "/")
	if strings.Contains(prefix, "\\") || (prefix != "" && (!fs.ValidPath(prefix) || prefix == ".")) {
		return errors.New("bunny: invalid library prefix")
	}
	return nil
}

type failedStorage struct{ err error }

func (s failedStorage) FS() (storage.MusicFS, error) { return nil, s.err }
func newStorage(u url.URL) storage.Storage {
	if err := validate(u); err != nil {
		return failedStorage{err}
	}
	origin := os.Getenv("BUNNY_STORAGE_ORIGIN")
	if origin == "" {
		origin = "https://storage.bunnycdn.com"
	}
	s, err := bunnystore.New(bunnystore.Config{
		Origin: origin, Zone: os.Getenv("BUNNY_STORAGE_ZONE"),
		Prefix: strings.TrimPrefix(u.Path, "/"), AccessKey: os.Getenv("BUNNY_STORAGE_ACCESS_KEY"),
	})
	if err != nil {
		return failedStorage{errors.New("bunny: storage configuration is invalid")}
	}
	return s
}
func init() { storage.Register("bunny", newStorage) }
