package family

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func MetadataFindings(t DoctorTrack) []DoctorFinding {
	out := []DoctorFinding{}
	add := func(severity, code, evidence string) {
		out = append(out, DoctorFinding{TrackID: t.ID, LibraryID: t.LibraryID, Severity: severity, Code: code, Evidence: evidence})
	}
	unknown := func(s string) bool {
		s = strings.ToLower(strings.TrimSpace(s))
		return s == "" || strings.HasPrefix(s, "[unknown")
	}
	if unknown(t.Title) {
		add("warning", "missing_title", "Catalog title is empty or unknown.")
	}
	if t.FilenameTitle {
		add("info", "filename_title", "Catalog title resembles the filename; this may be a fallback rather than an embedded title tag.")
	}
	if unknown(t.Artist) {
		add("warning", "missing_artist", "Catalog artist is empty or unknown.")
	}
	if unknown(t.Album) {
		add("warning", "missing_album", "Catalog album is empty or unknown.")
	}
	if t.Year <= 0 {
		add("info", "missing_year", "No release year is known. This is not an audio-quality score.")
	}
	if t.RecordingID == "" {
		add("info", "missing_recording_id", "MusicBrainz recording ID is not tagged; this optional identifier is not proof of quality.")
	}
	if t.CoverInvalid {
		add("warning", "unresolved_cover_reference", "A catalog cover reference was last resolved as absent. It may be stale or offline; no live storage probe or catalog deletion was performed.")
	}
	if !t.HasCover {
		add("info", "missing_cover", "Catalog has no cover reference; no remote artwork probe was performed.")
	}
	if t.TrackNumber <= 0 {
		add("info", "missing_track_number", "Track number is not available in the catalog.")
	}
	if t.Missing {
		add("warning", "catalog_missing", "Catalog marked this file missing. It may be offline; no original or catalog entry was removed.")
	}
	known := map[string]bool{"mp3": true, "flac": true, "wav": true, "m4a": true, "mp4": true, "ogg": true, "opus": true, "aac": true, "aiff": true, "aif": true, "ape": true, "wv": true, "wma": true, "dsf": true, "dff": true, "mpc": true, "tta": true, "alac": true}
	suffix, codec := strings.ToLower(t.Suffix), strings.ToLower(t.Codec)
	if !known[suffix] {
		add("info", "unusual_extension", "Extension is outside the common audio set; inspect the container before assuming damage.")
	}
	mismatch := codec != "" && (suffix == "flac" && codec != "flac" || suffix == "mp3" && codec != "mp3" || suffix == "m4a" && codec != "aac" && codec != "alac" && codec != "mp3")
	if mismatch {
		add("warning", "codec_container_mismatch", "Extracted codec is unusual for this extension. This is metadata evidence, not a decode failure.")
	}
	return out
}

// AcceptMetadataPage stores bounded SQL-only audit progress. The caller obtains
// its page from the authorized catalog; no storage reader is part of this API.
func (d Doctor) AcceptMetadataPage(ctx context.Context, key string, tracks []DoctorTrack, last bool, albumFindings []DoctorFinding) (DoctorJob, error) {
	x, e := d.authorize(ctx, key)
	if e != nil {
		return x, e
	}
	if x.Mode != "metadata" || x.State != "queued" && x.State != "active" || len(tracks) > 200 || len(albumFindings) > 400 || x.Processed+len(tracks) > x.Settings.MaxRows {
		return x, ErrConflict
	}
	previous := x.Cursor
	for _, t := range tracks {
		if t.ID <= previous {
			return x, ErrInvalid
		}
		previous = t.ID
	}
	tx, e := d.DB.BeginTx(ctx, nil)
	if e != nil {
		return x, e
	}
	defer tx.Rollback()
	now := d.now().Format(time.RFC3339Nano)
	var count int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_doctor_item`).Scan(&count); e != nil {
		return x, e
	}
	if count+len(tracks) > DoctorMaxItems {
		return x, ErrQuota
	}
	for _, track := range tracks {
		if track.AlbumAudited && track.AlbumID != "" {
			if _, e = tx.ExecContext(ctx, `INSERT OR IGNORE INTO family_doctor_album_audit(job_id,album_id) VALUES(?,?)`, key, track.AlbumID); e != nil {
				return x, e
			}
		}
		raw, e := json.Marshal(track)
		if e != nil || len(raw) > 4096 {
			return x, ErrInvalid
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO family_doctor_item(job_id,track_id,library_id,snapshot,state,updated_at) VALUES(?,?,?,?,'completed',?)`, key, track.ID, track.LibraryID, string(raw), now); e != nil {
			return x, e
		}
		for _, finding := range MetadataFindings(track) {
			if e = insertFinding(ctx, tx, key, track, finding, now); e != nil {
				return x, e
			}
		}
		// Exact canonical-path matches use a hash of the logical catalog path. Titles
		// and artist strings never establish duplicate audio evidence.
		if track.PathHash != "" {
			var existing string
			e = tx.QueryRowContext(ctx, `SELECT track_id FROM family_doctor_item WHERE job_id=? AND track_id<>? AND json_extract(snapshot,'$.pathHash')=? LIMIT 1`, key, track.ID, track.PathHash).Scan(&existing)
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return x, e
			}
			if e == nil {
				if e = insertFinding(ctx, tx, key, track, DoctorFinding{Severity: "warning", Code: "canonical_path_duplicate", Evidence: "Same canonical catalog path as recording " + existing + ". Byte identity has not been checked."}, now); e != nil {
					return x, e
				}
			}
		}
	}
	for _, f := range albumFindings {
		if len(f.Evidence) > 2000 || f.Severity != "warning" && f.Severity != "info" {
			return x, ErrInvalid
		}
		if e = insertFinding(ctx, tx, key, DoctorTrack{ID: f.TrackID, LibraryID: f.LibraryID}, f, now); e != nil {
			return x, e
		}
	}
	state := "active"
	if last || x.Processed+len(tracks) >= x.Settings.MaxRows {
		state = "completed"
	}
	res, e := tx.ExecContext(ctx, `UPDATE family_doctor_job SET state=?,cursor=?,processed=processed+?,updated_at=? WHERE id=? AND cursor=? AND state IN ('queued','active')`, state, previous, len(tracks), now, key, x.Cursor)
	if e != nil {
		return x, e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return x, ErrConflict
	}
	if e = tx.Commit(); e != nil {
		return x, e
	}
	return d.Get(ctx, key)
}
