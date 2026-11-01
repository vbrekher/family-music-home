package model

import (
	"net/url"
	"testing"
)

func TestBunnyMediaPathLiteralEscaping(t *testing.T) {
	for _, name := range []string{"Album/100% song.mp3", "Album/a%23b.mp3", "Album/a#b.mp3", "Album/a%2Fb.mp3"} {
		mf := MediaFile{LibraryPath: "bunny://zone/music", Path: name}
		raw := mf.AbsolutePath()
		u, e := url.Parse(raw)
		if e != nil || u.Path != "/music/"+name || u.RawQuery != "" || u.Fragment != "" {
			t.Fatalf("literal %q resolved incorrectly: %q", name, raw)
		}
	}
}
