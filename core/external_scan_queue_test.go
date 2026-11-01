package core

import (
	"context"
	"github.com/navidrome/navidrome/model"
	"sync/atomic"
	"testing"
	"time"
)

type queuedScanner struct {
	calls atomic.Int32
	done  chan struct{}
}

func (q *queuedScanner) ScanAll(context.Context, bool) ([]string, error) {
	panic("must target one external library")
}
func (q *queuedScanner) ScanFolders(ctx context.Context, _ bool, targets []model.ScanTarget) ([]string, error) {
	if ctx.Err() != nil {
		panic("HTTP cancellation leaked into queued scan")
	}
	if len(targets) != 1 || targets[0].LibraryID != 42 {
		panic("wrong library")
	}
	if q.calls.Add(1) == 1 {
		return nil, model.ErrAlreadyScanning
	}
	close(q.done)
	return nil, nil
}
func (q *queuedScanner) Status(context.Context) (*model.ScannerStatus, error) {
	return &model.ScannerStatus{}, nil
}
func TestExternalScanQueueRetriesBusyAndDetachesRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	q := &queuedScanner{done: make(chan struct{})}
	for i := 0; i < 20; i++ {
		queueExternalLibraryScan(ctx, q, 42)
	}
	select {
	case <-q.done:
	case <-time.After(4 * time.Second):
		t.Fatal("queued scan lost")
	}
	if q.calls.Load() != 2 {
		t.Fatalf("duplicate queued requests: %d", q.calls.Load())
	}
}
