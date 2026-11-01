package artwork

import (
	"context"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/tests"
	"testing"
)

func TestProtectedAccountDoesNotEnqueueExternalArtworkResolution(t *testing.T) {
	s := &service{ds: &tests.MockDataStore{}}
	ctx := request.WithUser(context.Background(), model.User{ID: "child0001", FamilyChild: &model.FamilyChild{Enabled: true}})
	// No queue repository is provided: touching it would panic instead of silently scheduling a provider.
	s.enqueue(ctx, model.ArtworkID{Kind: model.KindAlbumArtwork, ID: "album0001"}, model.ArtworkPriorityBump)
}
