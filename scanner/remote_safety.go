package scanner

import (
	"github.com/navidrome/navidrome/model"
)

// Remote listings are not authoritative transactional snapshots. Absence must
// not remove remote catalog records. Local storage retains upstream reconciliation.
func mayReconcileMissing(lib model.Library) bool {
	return !model.IsRemotePath(lib.Path)
}

func (j *scanJob) recordRemoteWalkError(err error) {
	if err == nil || mayReconcileMissing(j.lib) {
		return
	}
	j.lock.Lock()
	defer j.lock.Unlock()
	if j.walkErr == nil {
		j.walkErr = err
	}
}
