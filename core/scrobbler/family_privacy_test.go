package scrobbler

import (
	"context"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"testing"
	"time"
)

func TestProtectedAccountDoesNotDispatchExternalListeningData(t *testing.T) {
	p := &playTracker{}
	ctx := request.WithUser(context.Background(), model.User{ID: "child0001", FamilyChild: &model.FamilyChild{Enabled: true}})
	track := &model.MediaFile{ID: "track0001", Artist: "Synthetic artist"}
	// No plugin loader or external adapters exist; a dispatch path would panic if reached.
	p.dispatchScrobble(ctx, track, time.Now(), false)
	p.dispatchNowPlaying(ctx, "child0001", track, 1)
	p.dispatchPlaybackReport(ctx, PlaybackSession{Protected: true}, map[string]Scrobbler{"unusable": nil}, false)
}
