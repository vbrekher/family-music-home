package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/conf"
)

func TestPublicSetupRequiresPrivateLocalToken(t *testing.T) {
	old := conf.Server.DataFolder
	conf.Server.DataFolder = conf.NewDir(t.TempDir())
	t.Cleanup(func() { conf.Server.DataFolder = old })
	path := filepath.Join(conf.Server.DataFolder.String(), "setup-token")
	token := strings.Repeat("a", 43)
	req := httptest.NewRequest("POST", "/auth/createAdmin", nil)
	req.Header.Set("X-Family-Setup-Token", token)
	if authorizeFirstRun(req, "arbitrary-admin", "a strong local password") == nil {
		t.Fatal("missing setup token accepted")
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := authorizeFirstRun(req, "arbitrary-admin", "a strong local password"); err != nil {
		t.Fatal(err)
	}
	if err := authorizeFirstRun(req, "Мій-адміністратор", "a strong local password"); err != nil {
		t.Fatal(err)
	}
	if authorizeFirstRun(req, "admin\nforged-log", "a strong local password") == nil {
		t.Fatal("control character accepted in username")
	}
	req.Header.Set("X-Family-Setup-Token", strings.Repeat("b", 43))
	if authorizeFirstRun(req, "arbitrary-admin", "a strong local password") == nil {
		t.Fatal("wrong token accepted")
	}
	req.Header.Set("X-Family-Setup-Token", token)
	if authorizeFirstRun(req, "arbitrary-admin", "short") == nil {
		t.Fatal("weak password accepted")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if authorizeFirstRun(req, "arbitrary-admin", "a strong local password") == nil {
		t.Fatal("world-readable setup token accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(other, []byte(token), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, path); err != nil {
		t.Fatal(err)
	}
	if authorizeFirstRun(req, "arbitrary-admin", "a strong local password") == nil {
		t.Fatal("symlink token accepted")
	}
}
