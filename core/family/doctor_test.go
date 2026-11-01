package family

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/navidrome/navidrome/model/request"
)

func doctorFixture(t *testing.T) (Doctor, context.Context, context.Context, *time.Time, *bool) {
	s, o, w := collectorFixture(t)
	now := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	allowed := true
	d := Doctor{DB: s.DB, Now: func() time.Time { return now }, Track: func(ctx context.Context, key string) (DoctorTrack, error) {
		u, _ := request.UserFrom(ctx)
		if !u.IsAdmin && !allowed {
			return DoctorTrack{}, ErrDenied
		}
		return DoctorTrack{ID: key, LibraryID: 1, Library: "Test library", Title: "Same title is not duplicate proof", Size: 10, Version: "v1", Suffix: "flac", SampleRate: 44100, Available: true}, nil
	}}
	return d, o, w, &now, &allowed
}
func deepSettings(budget int64) DoctorSettings {
	return DoctorSettings{SourceBudgets: map[int]int64{1: budget}, Quality: true, Fingerprint: true, DecodeSeconds: 60, Confirmed: true}
}
func digest(data string) string { h := sha256.Sum256([]byte(data)); return hex.EncodeToString(h[:]) }
func TestDoctorReadBudgetSingleLeaseAndVerifiedDuplicateEvidence(t *testing.T) {
	d, o, w, _, allowed := doctorFixture(t)
	if _, e := d.Create(w, "deep", deepSettings(20), []string{"track0001"}); !errors.Is(e, ErrDenied) {
		t.Fatal("member started expensive work", e)
	}
	settings := deepSettings(20)
	settings.Confirmed = false
	if _, e := d.Create(o, "deep", settings, []string{"track0001"}); !errors.Is(e, ErrInvalid) {
		t.Fatal("missing explicit I/O approval accepted", e)
	}
	job, e := d.Create(o, "deep", deepSettings(20), []string{"track0001", "track0002"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = d.Get(w, job.ID); !errors.Is(e, ErrDenied) {
		t.Fatal("member read admin diagnostics", e)
	}
	if _, e = d.Create(o, "deep", deepSettings(20), []string{"track0003"}); !errors.Is(e, ErrConflict) {
		t.Fatal("parallel expensive jobs accepted", e)
	}
	hash := digest("abcdefghij")
	for i := 0; i < 2; i++ {
		item, e := d.Claim(o, job.ID)
		if e != nil || item == nil || item.State != "leased" {
			t.Fatal(item, e)
		}
		if second, e := d.Claim(o, job.ID); e != nil || second != nil {
			t.Fatal("parallel lease accepted", second, e)
		}
		if _, e = d.BeginFetch(o, job.ID, item.Track.ID, item.Lease); e != nil {
			t.Fatal(e)
		}
		if _, e = d.BeginFetch(o, job.ID, item.Track.ID, item.Lease); !errors.Is(e, ErrConflict) {
			t.Fatal("content request replay accepted", e)
		}
		if e = d.FinishFetch(o, job.ID, item.Track.ID, item.Lease, 10, hash, true); e != nil {
			t.Fatal(e)
		}
		result := DoctorResult{Worker: "local-test", SHA256: hash, Decode: "decoded", Duration: 1, Samples: 44100, NearFullScale: 2, Peak: 1}
		if e = d.Complete(o, job.ID, item.Track.ID, item.Lease, result); e != nil {
			t.Fatal(e)
		}
		if e = d.Complete(o, job.ID, item.Track.ID, item.Lease, result); e != nil {
			t.Fatal("result retry not idempotent", e)
		}
	}
	if item, e := d.Claim(o, job.ID); e != nil || item != nil {
		t.Fatal(item, e)
	}
	job, e = d.Get(o, job.ID)
	if e != nil || job.State != "completed" || job.Budgets[0].Reserved != 20 {
		t.Fatal(job, e)
	}
	groups, e := d.Duplicates(o, job.ID)
	if e != nil || len(groups) != 1 || groups[0].Kind != "exact-bytes" {
		t.Fatal(groups, e)
	}
	analysis, e := d.PassportAnalysis(w, "track0001")
	if e != nil || analysis["decode"] != "decoded" {
		t.Fatal(analysis, e)
	}
	*allowed = false
	if _, e = d.PassportAnalysis(w, "track0001"); !errors.Is(e, ErrDenied) {
		t.Fatal("passport evidence bypassed library authorization", e)
	}
}
func TestDoctorPauseCancelExpiryAndConservativeRetryBudget(t *testing.T) {
	d, o, _, now, _ := doctorFixture(t)
	job, e := d.Create(o, "deep", deepSettings(10), []string{"track0001"})
	if e != nil {
		t.Fatal(e)
	}
	item, e := d.Claim(o, job.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = d.BeginFetch(o, job.ID, item.Track.ID, item.Lease); e != nil {
		t.Fatal(e)
	}
	if _, e = d.Control(o, job.ID, "pause", false); e != nil {
		t.Fatal(e)
	}
	if e = d.LeaseActive(o, job.ID, item.Track.ID, item.Lease); !errors.Is(e, ErrConflict) {
		t.Fatal("pause did not stop stream", e)
	}
	if _, e = d.Control(o, job.ID, "resume", false); e != nil {
		t.Fatal(e)
	}
	next, e := d.Claim(o, job.ID)
	if e != nil || next == nil || next.State != "budget" {
		t.Fatal("retry silently exceeded payload budget", next, e)
	}
	if _, e = d.RaiseBudgets(o, job.ID, map[int]int64{1: 30}); e != nil {
		t.Fatal(e)
	}
	if _, e = d.Control(o, job.ID, "resume", true); e != nil {
		t.Fatal(e)
	}
	item, e = d.Claim(o, job.ID)
	if e != nil || item == nil {
		t.Fatal(item, e)
	}
	*now = now.Add(21 * time.Minute)
	expired, e := d.Claim(o, job.ID)
	if e != nil || expired == nil || expired.Lease == item.Lease || expired.Attempts != 3 {
		t.Fatal("expired companion lease not resumed safely", expired, e)
	}
	if _, e = d.Control(o, job.ID, "cancel", false); e != nil {
		t.Fatal(e)
	}
	if _, e = d.Claim(o, job.ID); !errors.Is(e, ErrConflict) {
		t.Fatal("cancelled job restarted", e)
	}
	if _, e = d.Control(o, job.ID, "resume", true); !errors.Is(e, ErrConflict) {
		t.Fatal("cancelled job resurrected", e)
	}
}
func TestDoctorMetadataAuditIsPagedAndDoesNotNeedAudioReader(t *testing.T) {
	d, o, _, _, _ := doctorFixture(t)
	job, e := d.Create(o, "metadata", DoctorSettings{MaxRows: 2}, nil)
	if e != nil {
		t.Fatal(e)
	}
	tracks := []DoctorTrack{{ID: "track0001", LibraryID: 1, Title: "", Artist: "[Unknown Artist]", Album: "", Suffix: "flac", Codec: "mp3", PathHash: "same-logical-path"}, {ID: "track0002", LibraryID: 1, PathHash: "same-logical-path"}}
	job, e = d.AcceptMetadataPage(o, job.ID, tracks, true, nil)
	if e != nil || job.State != "completed" || job.Processed != 2 {
		t.Fatal(job, e)
	}
	findings, e := d.Findings(o, job.ID, "", 0)
	if e != nil || len(findings) == 0 {
		t.Fatal(findings, e)
	}
	codes := map[string]bool{}
	for _, f := range findings {
		codes[f.Code] = true
	}
	if !codes["canonical_path_duplicate"] || !codes["codec_container_mismatch"] || !codes["missing_artist"] {
		t.Fatal(codes)
	}
}
func TestFingerprintsAreEvidenceNotTitleMatching(t *testing.T) {
	a := []uint32{}
	for i := 0; i < 60; i++ {
		a = append(a, uint32(i)*0x7f348ab1)
	}
	b := append([]uint32{}, a...)
	b[3] ^= 3
	if score := FingerprintSimilarity(a, b); score < .95 {
		t.Fatal(score)
	}
	flat := make([]uint32, 60)
	if score := FingerprintSimilarity(flat, flat); score != 0 {
		t.Fatal("low-entropy fingerprint accepted", score)
	}
	if score := FingerprintSimilarity(a[:5], b[:5]); score != 0 {
		t.Fatal("too little evidence accepted", score)
	}
}

func TestDoctorBoundedMetadataContinuationAndReportRemovalPreserveTrackEvidence(t *testing.T) {
	d, o, _, _, _ := doctorFixture(t)
	first, e := d.Create(o, "metadata", DoctorSettings{MaxRows: 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	first, e = d.AcceptMetadataPage(o, first.ID, []DoctorTrack{{ID: "track0001", LibraryID: 1}}, true, nil)
	if e != nil {
		t.Fatal(e)
	}
	next, e := d.Create(o, "metadata", DoctorSettings{MaxRows: 1, AfterID: first.Cursor}, nil)
	if e != nil || next.Cursor != first.Cursor {
		t.Fatal(next, e)
	}
	if _, e = d.AcceptMetadataPage(o, next.ID, []DoctorTrack{{ID: "track0001", LibraryID: 1}}, true, nil); !errors.Is(e, ErrInvalid) {
		t.Fatal("continuation repeated cursor", e)
	}
	if _, e = d.AcceptMetadataPage(o, next.ID, []DoctorTrack{{ID: "track0002", LibraryID: 1}}, true, nil); e != nil {
		t.Fatal(e)
	}
	if e = d.RemoveReport(o, first.ID, false); !errors.Is(e, ErrInvalid) {
		t.Fatal("diagnostic removal lacked confirmation", e)
	}
	if e = d.RemoveReport(o, first.ID, true); e != nil {
		t.Fatal(e)
	}
	if _, e = d.Get(o, first.ID); !errors.Is(e, ErrDenied) {
		t.Fatal("finished report still addressable", e)
	}
	// Deleting an audit run has no statement capable of deleting original music.
	if _, e = d.Get(o, next.ID); e != nil {
		t.Fatal("other run removed", e)
	}
}

func TestDoctorFirstUnavailableAttemptIsNotDisplayedAsNeverAnalyzed(t *testing.T) {
	d, o, w, _, _ := doctorFixture(t)
	job, e := d.Create(o, "deep", deepSettings(10), []string{"track0001"})
	if e != nil {
		t.Fatal(e)
	}
	item, e := d.Claim(o, job.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = d.BeginFetch(o, job.ID, item.Track.ID, item.Lease); e != nil {
		t.Fatal(e)
	}
	if e = d.FinishFetch(o, job.ID, item.Track.ID, item.Lease, 0, "", false); e != nil {
		t.Fatal(e)
	}
	value, e := d.PassportAnalysis(w, item.Track.ID)
	if e != nil || value["hasSuccessfulEvidence"] != false || value["latestAttemptState"] != "unavailable" || value["evidenceStaleOrIncomplete"] != true {
		t.Fatal(value, e)
	}
}
