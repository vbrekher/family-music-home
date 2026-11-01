package family

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"math/bits"
	"sort"
)

type DoctorDuplicate struct {
	Kind       string   `json:"kind"`
	TrackIDs   []string `json:"trackIds"`
	Evidence   string   `json:"evidence"`
	Similarity *float64 `json:"similarity,omitempty"`
}

// FingerprintSimilarity compares aligned Chromaprint bit patterns. A high value
// supports a probable recording match, never identical mastering/provenance.
func FingerprintSimilarity(a, b []uint32) float64 {
	if len(a) < 20 || len(b) < 20 {
		return 0
	}
	unique := map[uint32]bool{}
	for _, v := range a {
		unique[v] = true
	}
	if len(unique) < 8 {
		return 0
	}
	best := 0.0
	for shift := -10; shift <= 10; shift++ {
		different, count := 0, 0
		for i, v := range a {
			j := i + shift
			if j < 0 || j >= len(b) {
				continue
			}
			different += bits.OnesCount32(v ^ b[j])
			count++
			if count >= 1000 {
				break
			}
		}
		if count < 20 {
			continue
		}
		score := 1 - float64(different)/float64(count*32)
		if score > best {
			best = score
		}
	}
	return best
}
func (d Doctor) Duplicates(ctx context.Context, key string) ([]DoctorDuplicate, error) {
	if _, e := d.authorize(ctx, key); e != nil {
		return nil, e
	}
	rows, e := d.DB.QueryContext(ctx, `SELECT track_id,sha256,result FROM family_doctor_item WHERE job_id=? AND state='completed' AND sha256<>'' ORDER BY track_id LIMIT 100`, key)
	if e != nil {
		return nil, e
	}
	type entry struct {
		id, hash string
		result   DoctorResult
	}
	items := []entry{}
	hashes := map[string][]string{}
	for rows.Next() {
		var a entry
		var raw string
		if e = rows.Scan(&a.id, &a.hash, &raw); e != nil {
			rows.Close()
			return nil, e
		}
		if e = json.Unmarshal([]byte(raw), &a.result); e != nil {
			rows.Close()
			return nil, e
		}
		items = append(items, a)
		hashes[a.hash] = append(hashes[a.hash], a.id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []DoctorDuplicate{}
	keys := []string{}
	for hash := range hashes {
		keys = append(keys, hash)
	}
	sort.Strings(keys)
	for _, hash := range keys {
		if len(hashes[hash]) > 1 {
			out = append(out, DoctorDuplicate{Kind: "exact-bytes", TrackIDs: hashes[hash], Evidence: "Streamed SHA-256 matches: " + hash + ". No original was changed."})
		}
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			a, b := items[i], items[j]
			if a.hash == b.hash || a.result.FingerprintDuration < 8 || b.result.FingerprintDuration < 8 {
				continue
			}
			if math.Abs(a.result.FingerprintDuration-b.result.FingerprintDuration) > 5 {
				continue
			}
			score := FingerprintSimilarity(a.result.Fingerprint, b.result.Fingerprint)
			if score >= 0.88 {
				out = append(out, DoctorDuplicate{Kind: "probable-recording", TrackIDs: []string{a.id, b.id}, Evidence: "Similar local Chromaprint evidence. Different encoding, edits or mastering may still exist; matching titles are not used.", Similarity: &score})
			}
		}
	}
	return out, nil
}

// PassportAnalysis intentionally excludes job selections, worker notes and raw
// fingerprints. The caller's current track authorization remains canonical.
func (d Doctor) PassportAnalysis(ctx context.Context, key string) (map[string]any, error) {
	if _, e := actor(ctx); e != nil {
		return nil, e
	}
	if d.DB == nil {
		return nil, sql.ErrNoRows
	}
	track, e := d.Track(ctx, key)
	if e != nil {
		return nil, e
	}
	var version, hash, raw, at string
	e = d.DB.QueryRowContext(ctx, `SELECT catalog_version,sha256,result,verified_at FROM family_doctor_analysis WHERE track_id=? AND library_id=?`, key, track.LibraryID).Scan(&version, &hash, &raw, &at)
	hasSuccess := e == nil
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	var result DoctorResult
	if hasSuccess {
		if e = json.Unmarshal([]byte(raw), &result); e != nil {
			return nil, e
		}
	}
	var latestState, latestDecode, latestHash, latestAt, latestRaw, latestVersion string
	e = d.DB.QueryRowContext(ctx, `SELECT i.state,coalesce(json_extract(i.result,'$.decode'),''),i.sha256,i.updated_at,i.result,json_extract(i.snapshot,'$.version') FROM family_doctor_item i JOIN family_doctor_job j ON j.id=i.job_id WHERE i.track_id=? AND i.library_id=? AND i.attempts>0 AND j.mode='deep' ORDER BY i.updated_at DESC LIMIT 1`, key, track.LibraryID).Scan(&latestState, &latestDecode, &latestHash, &latestAt, &latestRaw, &latestVersion)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	if !hasSuccess && errors.Is(e, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if !hasSuccess {
		if e = json.Unmarshal([]byte(latestRaw), &result); e != nil {
			return nil, e
		}
		version = latestVersion
		hash = latestHash
		if latestState == "completed" && latestDecode != "error" && latestDecode != "budget" {
			hasSuccess = true
			at = latestAt
		}
	}
	stale := !hasSuccess || version != track.Version || latestHash != "" && latestHash != hash || latestState == "unavailable" || latestState == "failed" || latestDecode == "error" || latestDecode == "budget"
	return map[string]any{"hasSuccessfulEvidence": hasSuccess, "catalogChangedSinceAnalysis": version != track.Version, "evidenceStaleOrIncomplete": stale, "verifiedAt": at, "sha256": hash, "decode": result.Decode, "nearFullScaleSamples": result.NearFullScale, "measuredSamples": result.Samples, "upperBandRatio": result.UpperBandRatio, "measuredCodec": result.MeasuredCodec, "measuredSampleRate": result.MeasuredSampleRate, "latestAttemptState": latestState, "latestAttemptDecode": latestDecode, "latestAttemptAt": latestAt, "provenance": "Unknown; spectrum and metadata do not prove ancestry"}, nil
}
