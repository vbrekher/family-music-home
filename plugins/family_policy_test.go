package plugins

import (
	"context"
	"net/url"
	"testing"
)

func TestFamilyPluginSafety(t *testing.T) {
	if validateFamilyConfig("nd-lyrics", `{"writeLyrics":true}`) == nil {
		t.Fatal("sidecar write accepted")
	}
	if e := validateFamilyConfig("nd-lyrics", `{"writeLyrics":false}`); e != nil {
		t.Fatal(e)
	}
	if e := validateFamilyConfig("other-plugin", `{"writeLyrics":true}`); e != nil {
		t.Fatal("unrelated plugin changed")
	}
	u, _ := url.Parse("https://lrclib.net/api/get")
	if e := familyRequestLimit(context.Background(), "nd-lyrics", "POST", u); e == nil {
		t.Fatal("lyrics write request accepted")
	}
	u.Scheme = "http"
	if e := familyRequestLimit(context.Background(), "apple-music", "GET", u); e == nil {
		t.Fatal("unencrypted metadata request accepted")
	}
}
