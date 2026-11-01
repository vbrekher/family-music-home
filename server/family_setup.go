package server

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/navidrome/navidrome/conf"
)

// Serialize the count-and-create sequence, including first-run HTTP requests.
// Supported deployments run one application process per SQLite volume.
var initialAdminLock sync.Mutex

func publicSelfHost() bool { return os.Getenv("FMH_INSTALL_MODE") == "PUBLIC_SELF_HOST" }

func authorizeFirstRun(r *http.Request, username, password string) error {
	invalidName := strings.IndexFunc(username, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' && r != '-' && r != '.'
	}) >= 0
	if invalidName || len(username) < 1 || len(username) > 64 || utf8.RuneCountInString(password) < 12 || len(password) > 1024 {
		return errors.New("choose a username and a password of at least 12 characters")
	}
	path := filepath.Join(conf.Server.DataFolder.String(), "setup-token")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 128 {
		return errors.New("setup token unavailable; read the installation guide")
	}
	token, err := os.ReadFile(path)
	provided := r.Header.Get("X-Family-Setup-Token")
	if err != nil || len(token) < 32 || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(string(token))), []byte(provided)) != 1 {
		return errors.New("enter the setup token from your server's data folder")
	}
	return nil
}
