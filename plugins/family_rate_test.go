package plugins

import (
	"context"
	"net/url"
	"testing"
)

func TestFamilyProviderTransportRestrictions(t *testing.T) {
	for _, raw := range []string{"http://lrclib.net/api/get", "https://archive.org/other", "https://evil.archive.org/0/items/mbid-abcd/index.json", "https://dn720706.ca.archive.org/0/items/private/index.json"} {
		u, _ := url.Parse(raw)
		if familyRequestLimit(context.Background(), "coverartarchive", "GET", u) == nil {
			t.Errorf("accepted unsafe URL %s", raw)
		}
	}
	for _, raw := range []string{"https://archive.org/download/mbid-30702389-5c67-4438-9ea0-2351c8de0f1d/index.json", "https://dn720706.ca.archive.org/0/items/mbid-30702389-5c67-4438-9ea0-2351c8de0f1d/index.json"} {
		u, _ := url.Parse(raw)
		if err := familyRequestLimit(context.Background(), "coverartarchive", "GET", u); err != nil {
			t.Error(err)
		}
	}
	u, _ := url.Parse("https://lrclib.net/api/get")
	if familyRequestLimit(context.Background(), "nd-lyrics", "POST", u) == nil {
		t.Error("accepted write method")
	}
}
