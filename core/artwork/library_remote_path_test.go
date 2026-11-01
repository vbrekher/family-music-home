package artwork

import (
	"net/url"
	"testing"
)

func TestBunnyArtworkPathLiteralEscaping(t *testing.T) {
	view := libraryView{absRoot: "bunny://zone/music"}
	for _, name := range []string{"Album/100% art.jpg", "Album/a%23b.jpg", "Album/a#b.jpg", "Album/a%2Fb.jpg"} {
		raw := view.Abs(name)
		u, e := url.Parse(raw)
		if e != nil || u.Path != "/music/"+name || u.RawQuery != "" || u.Fragment != "" {
			t.Fatalf("literal %q resolved incorrectly: %q", name, raw)
		}
	}
}
