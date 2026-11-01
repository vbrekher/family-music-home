package scanner

import (
	"context"
	"errors"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/metadata"
	"io/fs"
	"testing"
)

type remoteSafetyStore struct{ model.DataStore }

func (s remoteSafetyStore) WithTxRetry(ctx context.Context, f func(context.Context, model.DataStore) error, _ ...string) error {
	return f(ctx, s)
}

func TestRemoteMissingReconciliationPolicy(t *testing.T) {
	for _, p := range []string{"bunny://zone/music", "bunny://zone", "s3://archive/music", "webdav://nas/music", "sftp://nas/music"} {
		if mayReconcileMissing(model.Library{Path: p}) {
			t.Fatal("remote reconciliation enabled", p)
		}
	}
	for _, p := range []string{"/music", "file:///music", "relative/music", "https://example.com/music", "testFS://fixture/music"} {
		if !mayReconcileMissing(model.Library{Path: p}) {
			t.Fatal("local reconciliation disabled", p)
		}
	}
}
func TestRemoteMissingFoldersNeverTouchCatalog(t *testing.T) {
	// A failed/partial walk leaves known folders unvisited. Any datastore access
	// beyond the transaction callback panics through the nil embedded interface.
	ctx := context.Background()
	_, cancel := context.WithCancelCause(ctx)
	p := phaseFolders{ctx: ctx, ds: remoteSafetyStore{}, stopWalk: cancel, imageChanges: &imageChangeCollector{}, jobs: []*scanJob{{lib: model.Library{Path: "bunny://zone/music"}, lastUpdates: map[string]model.FolderUpdateInfo{"unvisited": {}}}}}
	if e := p.finalize(nil); e != nil {
		t.Fatal(e)
	}
}
func TestRemoteMissingProducerNeverQueriesOrRemovesTracks(t *testing.T) {
	p := phaseMissingTracks{ctx: context.Background(), state: &scanState{libraries: []model.Library{{Path: "bunny://zone/music"}}}}
	if e := p.produce(func(*missingTracks) { t.Fatal("remote missing track emitted") }); e != nil {
		t.Fatal(e)
	}
}

type unavailableRemoteFS struct{}

func (unavailableRemoteFS) Open(string) (fs.File, error) { return nil, fs.ErrPermission }
func (unavailableRemoteFS) ReadTags(...string) (map[string]metadata.Info, error) {
	return nil, fs.ErrPermission
}
func TestRemoteWalkRecordsUnavailableOrigin(t *testing.T) {
	job := &scanJob{lib: model.Library{Path: "bunny://zone/music"}, fs: unavailableRemoteFS{}}
	ch, e := walkDirTree(context.Background(), job)
	if e != nil {
		t.Fatal(e)
	}
	for range ch {
		t.Fatal("outage emitted a valid folder")
	}
	if !errors.Is(job.walkErr, fs.ErrPermission) {
		t.Fatal("outage was swallowed", job.walkErr)
	}
}

type scanEndRepo struct {
	model.LibraryRepository
	ended []int
}

func (r *scanEndRepo) ScanEnd(id int) error { r.ended = append(r.ended, id); return nil }

type scanPropertyRepo struct{ model.PropertyRepository }

func (scanPropertyRepo) Put(string, string) error { return nil }

type isolatedScanStore struct {
	model.DataStore
	libs *scanEndRepo
}

func (s isolatedScanStore) WithTxRetry(c context.Context, f func(context.Context, model.DataStore) error, _ ...string) error {
	return f(c, s)
}
func (s isolatedScanStore) Library(context.Context) model.LibraryRepository { return s.libs }
func (s isolatedScanStore) Property(context.Context) model.PropertyRepository {
	return scanPropertyRepo{}
}
func TestUnavailableLibraryDoesNotCompleteOrBlockOtherLibraries(t *testing.T) {
	repo := &scanEndRepo{}
	state := &scanState{libraries: model.Libraries{{ID: 1, Path: "bunny://available/music"}, {ID: 2, Path: "webdav://offline"}, {ID: 3, Path: "s3://available"}}}
	state.markLibraryFailed(2)
	s := scannerImpl{ds: isolatedScanStore{libs: repo}}
	if err := s.runUpdateLibraries(context.Background(), state)(); err != nil {
		t.Fatal(err)
	}
	if len(repo.ended) != 2 || repo.ended[0] != 1 || repo.ended[1] != 3 {
		t.Fatalf("completed unavailable source or blocked healthy source: %v", repo.ended)
	}
}
