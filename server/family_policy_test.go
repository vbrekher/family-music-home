package server

import (
	"github.com/navidrome/navidrome/model"
	"testing"
)

func TestProtectedFamilyRouteFailClosedAcrossProtocols(t *testing.T) {
	child := model.User{ID: "child0001", FamilyChild: &model.FamilyChild{Enabled: true, ParentID: "parent001", Interface: "kids"}}
	for _, p := range []string{"/api/external-source", "/api/user", "/api/library", "/api/plugin", "/api/family/doctor/jobs", "/api/family/children", "/api/artist/artist001/info", "/rest/getLyrics.view", "/rest/getArtistInfo2.view", "/rest/startScan.view", "/rest/getNowPlaying.view", "/events", "/jellyfin/Items"} {
		if ProtectedFamilyRoute(child, p, "GET") {
			t.Fatal("protected path allowed", p)
		}
	}
	for _, p := range []string{"/api/song/track001", "/api/album", "/api/family/shelves", "/api/family/kids", "/rest/stream.view", "/rest/getCoverArt.view", "/rest/star.view", "/rest/getMusicDirectory.view"} {
		if !ProtectedFamilyRoute(child, p, "GET") {
			t.Fatal("own catalog path blocked", p)
		}
	}
	if !ProtectedFamilyRoute(child, "/rest/getTranscodeDecision", "POST") || ProtectedFamilyRoute(child, "/rest/getTranscodeDecision", "GET") {
		t.Fatal("bounded canonical playback negotiation policy")
	}
	if !ProtectedFamilyRoute(child, "/rest/getTranscodeStream", "GET") || !ProtectedFamilyRoute(child, "/rest/getTranscodeStream", "HEAD") || ProtectedFamilyRoute(child, "/rest/getTranscodeStream", "POST") {
		t.Fatal("canonical signed stream read policy")
	}
	if ProtectedFamilyRoute(child, "/api/song", "POST") {
		t.Fatal("catalog write allowed")
	}
	child.FamilyChild.Interface = "full"
	if ProtectedFamilyRoute(child, "/api/external-source", "GET") {
		t.Fatal("full interface elevated permissions")
	}
	child.FamilyChild.Enabled = false
	if ProtectedFamilyRoute(child, "/rest/stream.view", "GET") || ProtectedFamilyRoute(child, "/rest/getTranscodeStream", "GET") || !ProtectedFamilyRoute(child, "/api/family/me", "GET") {
		t.Fatal("paused policy")
	}
	if !ProtectedFamilyRoute(model.User{ID: "member"}, "/api/family/letters", "GET") {
		t.Fatal("adult policy changed")
	}
}
