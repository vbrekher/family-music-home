package bunny

import (
	"github.com/navidrome/navidrome/core/storage"
	"strings"
	"testing"
)

func TestValidateURI(t *testing.T) {
	t.Setenv("BUNNY_STORAGE_ZONE", "family")
	for _, uri := range []string{"bunny://family/music", "bunny://family", "bunny://family/music/Artist%20Name"} {
		if err := ValidateURI(uri); err != nil {
			t.Errorf("valid %q: %v", uri, err)
		}
	}
	for _, uri := range []string{"bunny://other/music", "bunny://user:secret@family/music", "bunny://family:443/music", "bunny://family/music?secret=x", "bunny://family/music#x", "bunny://family/../music", "bunny://family/%2e%2e/music", "bunny://family//music", "bunny://family/music/", "bunny://family/a%5Cb", "file://family/music"} {
		if ValidateURI(uri) == nil {
			t.Errorf("accepted %q", uri)
		}
	}
}
func TestRegistrationAndConfigurationFailure(t *testing.T) {
	t.Setenv("BUNNY_STORAGE_ZONE", "family")
	t.Setenv("BUNNY_STORAGE_ACCESS_KEY", "")
	s, err := storage.For("bunny://family/music")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.FS(); err == nil {
		t.Fatal("missing credential accepted")
	}
	t.Setenv("BUNNY_STORAGE_ACCESS_KEY", "do-not-log-this-secret")
	t.Setenv("BUNNY_STORAGE_ORIGIN", "https://user:do-not-log-this-secret@example.com")
	s, err = storage.For("bunny://family/music")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.FS(); err == nil || strings.Contains(err.Error(), "do-not-log") {
		t.Fatal("configuration failure missing or leaked a secret")
	}
	t.Setenv("BUNNY_STORAGE_ORIGIN", "")
	s, err = storage.For("bunny://family/music")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.FS(); err != nil {
		t.Fatal(err)
	}
}
