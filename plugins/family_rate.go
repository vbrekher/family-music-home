package plugins

import (
	"context"
	"fmt"
	"golang.org/x/time/rate"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var familyLimiters sync.Map

func familyRequestLimit(ctx context.Context, plugin, method string, u *url.URL) error {
	switch plugin {
	case "nd-lyrics", "apple-music", "coverartarchive":
	default:
		return nil
	}
	if u.Scheme != "https" || (method != "GET" && method != "HEAD") {
		return fmt.Errorf("family metadata plugins require read-only HTTPS")
	}
	if plugin == "coverartarchive" && (u.Hostname() == "archive.org" || strings.HasSuffix(u.Hostname(), ".archive.org")) {
		direct := u.Hostname() == "archive.org" && regexp.MustCompile(`^/download/mbid-[a-f0-9-]+/index\.json$`).MatchString(u.Path)
		shard := regexp.MustCompile(`^(?:ia|dn)[0-9]+\.[a-z]{2}\.archive\.org$`).MatchString(u.Hostname()) && regexp.MustCompile(`^/[0-9]+/items/mbid-[a-f0-9-]+/index\.json$`).MatchString(u.Path)
		if !direct && !shard {
			return fmt.Errorf("CAA archive redirect outside index scope")
		}
	}

	interval := 500 * time.Millisecond
	if u.Hostname() == "musicbrainz.org" {
		interval = time.Second
	}
	value, _ := familyLimiters.LoadOrStore(plugin+":"+u.Hostname(), rate.NewLimiter(rate.Every(interval), 1))
	return value.(*rate.Limiter).Wait(ctx)
}
