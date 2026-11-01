package family

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

// BeginFetch claims the one allowed content request. Pausing/cancelling or another
// lease makes every subsequent check fail, including during an active stream.
func (d Doctor) BeginFetch(ctx context.Context, key, trackID, lease string) (DoctorTrack, error) {
	if _, e := d.authorize(ctx, key); e != nil {
		return DoctorTrack{}, e
	}
	var snap string
	tx, e := d.DB.BeginTx(ctx, nil)
	if e != nil {
		return DoctorTrack{}, e
	}
	defer tx.Rollback()
	var state string
	e = tx.QueryRowContext(ctx, `SELECT i.snapshot,j.state FROM family_doctor_item i JOIN family_doctor_job j ON j.id=i.job_id WHERE i.job_id=? AND i.track_id=? AND i.lease=? AND i.state='leased' AND i.lease_until>?`, key, trackID, lease, d.now().UnixMilli()).Scan(&snap, &state)
	if errors.Is(e, sql.ErrNoRows) {
		return DoctorTrack{}, ErrConflict
	}
	if e != nil {
		return DoctorTrack{}, e
	}
	if state != "active" {
		return DoctorTrack{}, ErrConflict
	}
	var track DoctorTrack
	if e = json.Unmarshal([]byte(snap), &track); e != nil {
		return track, e
	}
	current, e := d.Track(ctx, trackID)
	if e != nil {
		return track, ErrDenied
	}
	if current.Version != track.Version || current.Size != track.Size || current.LibraryID != track.LibraryID {
		note, _ := json.Marshal(DoctorResult{Note: "Catalog changed since selection; create a fresh selection after reviewing metadata."})
		if _, e = tx.ExecContext(ctx, `UPDATE family_doctor_item SET state='changed',lease='',lease_until=0,result=?,updated_at=? WHERE job_id=? AND track_id=?`, string(note), d.now().Format(time.RFC3339Nano), key, trackID); e != nil {
			return track, e
		}
		if e = tx.Commit(); e != nil {
			return track, e
		}
		return track, ErrConflict
	}
	if _, e = tx.ExecContext(ctx, `UPDATE family_doctor_item SET state='fetching',updated_at=? WHERE job_id=? AND track_id=? AND lease=?`, d.now().Format(time.RFC3339Nano), key, trackID, lease); e != nil {
		return track, e
	}
	if e = tx.Commit(); e != nil {
		return track, e
	}
	return track, nil
}
func (d Doctor) LeaseActive(ctx context.Context, key, trackID, lease string) error {
	if _, e := d.authorize(ctx, key); e != nil {
		return e
	}
	var found int
	e := d.DB.QueryRowContext(ctx, `SELECT 1 FROM family_doctor_item i JOIN family_doctor_job j ON j.id=i.job_id WHERE i.job_id=? AND i.track_id=? AND i.lease=? AND i.state='fetching' AND i.lease_until>? AND j.state='active'`, key, trackID, lease, d.now().UnixMilli()).Scan(&found)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrConflict
	}
	return e
}
func (d Doctor) FinishFetch(ctx context.Context, key, trackID, lease string, n int64, digest string, complete bool) error {
	if _, e := d.authorize(ctx, key); e != nil {
		return e
	}
	if n < 0 {
		return ErrInvalid
	}
	state := "unavailable"
	if complete {
		state = "downloaded"
		if !validSHA(digest) {
			return ErrInvalid
		}
	}
	result := DoctorResult{Note: "Source unavailable or transfer incomplete; catalog and previous successful analysis preserved"}
	if complete {
		result = DoctorResult{}
	}
	raw, _ := json.Marshal(result)
	res, e := d.DB.ExecContext(ctx, `UPDATE family_doctor_item SET state=?,bytes_read=?,sha256=?,result=?,updated_at=? WHERE job_id=? AND track_id=? AND lease=? AND state='fetching' AND lease_until>? AND EXISTS(SELECT 1 FROM family_doctor_job WHERE id=? AND state='active') AND (?=0 OR json_extract(snapshot,'$.size')=?)`, state, n, digest, string(raw), d.now().Format(time.RFC3339Nano), key, trackID, lease, d.now().UnixMilli(), key, complete, n)
	if e != nil {
		return e
	}
	count, _ := res.RowsAffected()
	if count != 1 {
		return ErrConflict
	}
	return nil
}
func validSHA(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == sha256.Size && strings.ToLower(s) == s
}
func validNumber(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }
func (d Doctor) Complete(ctx context.Context, key, trackID, lease string, result DoctorResult) error {
	job, e := d.authorize(ctx, key)
	if e != nil {
		return e
	}
	var done int
	if e = d.DB.QueryRowContext(ctx, `SELECT count(*) FROM family_doctor_item WHERE job_id=? AND track_id=? AND lease=? AND state='completed' AND sha256=?`, key, trackID, lease, result.SHA256).Scan(&done); e != nil {
		return e
	}
	if done == 1 {
		return nil
	}
	if job.State != "active" {
		return ErrConflict
	}
	if len(result.MeasuredCodec) > 80 || result.MeasuredSampleRate < 0 || result.MeasuredSampleRate > 768000 || result.MeasuredChannels < 0 || result.MeasuredChannels > 8 {
		return ErrInvalid
	}
	if !validSHA(result.SHA256) || len(result.Worker) > 120 || len(result.Note) > 2000 || len(result.BandwidthNote) > 1000 || len(result.Fingerprint) > 4096 || len(result.SpectralBands) > 32 || !validNumber(result.Duration) || result.Duration > 24*3600 || !validNumber(result.Peak) || result.Peak > 100 || result.Samples < 0 || result.NearFullScale < 0 || result.NearFullScale > result.Samples || !validNumber(result.FingerprintDuration) || result.FingerprintDuration > 180 {
		return ErrInvalid
	}
	if result.UpperBandRatio != nil && (!validNumber(*result.UpperBandRatio) || *result.UpperBandRatio > 1) {
		return ErrInvalid
	}
	for _, n := range result.SpectralBands {
		if !validNumber(n) {
			return ErrInvalid
		}
	}
	if result.Decode == "decoded" && (result.Samples <= 0 || result.Duration <= 0) {
		return ErrInvalid
	}
	if result.Decode != "not-requested" && result.Decode != "decoded" && result.Decode != "error" && result.Decode != "unsupported" && result.Decode != "budget" {
		return ErrInvalid
	}
	if !job.Settings.Fingerprint && len(result.Fingerprint) > 0 || !job.Settings.Quality && result.Decode != "not-requested" {
		return ErrInvalid
	}
	raw, e := json.Marshal(result)
	if e != nil || len(raw) > 16384 {
		return ErrInvalid
	}
	tx, e := d.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var snap, hash, state string
	var size, read int64
	e = tx.QueryRowContext(ctx, `SELECT i.snapshot,i.sha256,json_extract(i.snapshot,'$.size'),i.bytes_read,j.state FROM family_doctor_item i JOIN family_doctor_job j ON j.id=i.job_id WHERE i.job_id=? AND i.track_id=? AND i.lease=? AND i.state='downloaded' AND i.lease_until>?`, key, trackID, lease, d.now().UnixMilli()).Scan(&snap, &hash, &size, &read, &state)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrConflict
	}
	if e != nil {
		return e
	}
	if state != "active" || hash != result.SHA256 || size != read {
		return ErrConflict
	}
	var track DoctorTrack
	if e = json.Unmarshal([]byte(snap), &track); e != nil {
		return e
	}
	now := d.now().Format(time.RFC3339Nano)
	if _, e = tx.ExecContext(ctx, `UPDATE family_doctor_item SET state='completed',result=?,lease_until=0,updated_at=? WHERE job_id=? AND track_id=?`, string(raw), now, key, trackID); e != nil {
		return e
	}
	// A new unavailable/failed attempt never erases the last successful evidence.
	if result.Decode != "error" && result.Decode != "budget" {
		var bytes, count, previous int64
		if e = tx.QueryRowContext(ctx, `SELECT coalesce(sum(length(CAST(result AS BLOB))),0),count(*),coalesce(sum(CASE WHEN track_id=? THEN length(CAST(result AS BLOB)) ELSE 0 END),0) FROM family_doctor_analysis`, track.ID).Scan(&bytes, &count, &previous); e != nil {
			return e
		}
		if bytes-previous+int64(len(raw)) <= 64*1024*1024 && (count < 50000 || previous > 0) {
			if _, e = tx.ExecContext(ctx, `INSERT INTO family_doctor_analysis(track_id,library_id,catalog_version,sha256,result,verified_at) VALUES(?,?,?,?,?,?) ON CONFLICT(track_id) DO UPDATE SET library_id=excluded.library_id,catalog_version=excluded.catalog_version,sha256=excluded.sha256,result=excluded.result,verified_at=excluded.verified_at WHERE json_extract(excluded.result,'$.decode')='decoded' OR coalesce(json_extract(family_doctor_analysis.result,'$.decode'),'')<>'decoded'`, track.ID, track.LibraryID, track.Version, hash, string(raw), now); e != nil {
				return e
			}
		} else {
			if e = insertFinding(ctx, tx, key, track, DoctorFinding{Severity: "info", Code: "passport_cache_quota", Evidence: "This run retains its evidence, but the bounded passport cache is full. Existing successful evidence was not overwritten."}, now); e != nil {
				return e
			}
		}
	}
	findings := []DoctorFinding{}
	if result.MeasuredCodec != "" && track.Codec != "" && result.MeasuredCodec != track.Codec {
		findings = append(findings, DoctorFinding{Severity: "warning", Code: "measured_codec_differs", Evidence: "Local probe codec differs from the cached catalog. Review tags/container and refresh metadata manually if appropriate."})
	}
	if result.MeasuredSampleRate > 0 && track.SampleRate > 0 && result.MeasuredSampleRate != track.SampleRate {
		findings = append(findings, DoctorFinding{Severity: "warning", Code: "measured_rate_differs", Evidence: "Local probe sample rate differs from cached metadata. This is a factual mismatch, not a quality ranking."})
	}
	if result.Decode == "error" {
		findings = append(findings, DoctorFinding{Severity: "error", Code: "decode_error", Evidence: "Strict local decoder reported an error. Inspect the original manually; no file was changed."})
	}
	if result.Decode == "budget" {
		findings = append(findings, DoctorFinding{Severity: "info", Code: "decode_budget", Evidence: "Local analysis deadline reached; integrity remains unknown."})
	}
	if result.NearFullScale > 0 {
		findings = append(findings, DoctorFinding{Severity: "warning", Code: "full_scale_samples", Evidence: "Repeated near-full-scale samples measured in decoded PCM. This is a clipping heuristic, not a verdict on audible quality."})
	}
	if result.UpperBandRatio != nil && *result.UpperBandRatio < 0.00001 && track.SampleRate >= 44100 {
		findings = append(findings, DoctorFinding{Severity: "info", Code: "limited_upper_band", Evidence: "Very little measured upper-band energy. Natural filtering or mastering may explain it; spectrum alone cannot prove lossy ancestry or upsampling."})
	}
	for _, f := range findings {
		if e = insertFinding(ctx, tx, key, track, f, now); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func insertFinding(ctx context.Context, tx *sql.Tx, key string, track DoctorTrack, f DoctorFinding, now string) error {
	limit := 0
	switch f.Severity {
	case "info":
		limit = 500
	case "warning":
		limit = 400
	case "error":
		limit = 100
	default:
		return ErrInvalid
	}
	_, e := tx.ExecContext(ctx, `INSERT INTO family_doctor_finding(job_id,track_id,library_id,severity,code,evidence,created_at) SELECT ?,?,?,?,?,?,? WHERE (SELECT count(*) FROM family_doctor_finding WHERE job_id=?)<? AND (SELECT count(*) FROM family_doctor_finding WHERE job_id=? AND severity=?)<?`, key, track.ID, track.LibraryID, f.Severity, f.Code, f.Evidence, now, key, DoctorMaxFindings, key, f.Severity, limit)
	return e
}
func (d Doctor) Findings(ctx context.Context, key, severity string, offset int) ([]DoctorFinding, error) {
	if _, e := d.authorize(ctx, key); e != nil {
		return nil, e
	}
	if offset < 0 || offset > 10000 || severity != "" && severity != "info" && severity != "warning" && severity != "error" {
		return nil, ErrInvalid
	}
	query := `SELECT id,track_id,library_id,severity,code,evidence,created_at FROM family_doctor_finding WHERE job_id=?`
	args := []any{key}
	if severity != "" {
		query += ` AND severity=?`
		args = append(args, severity)
	}
	query += ` ORDER BY id LIMIT 100 OFFSET ?`
	args = append(args, offset)
	rows, e := d.DB.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []DoctorFinding{}
	for rows.Next() {
		var f DoctorFinding
		if e = rows.Scan(&f.ID, &f.TrackID, &f.LibraryID, &f.Severity, &f.Code, &f.Evidence, &f.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
