package family

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/navidrome/navidrome/model/id"
)

func (d Doctor) Create(ctx context.Context, mode string, settings DoctorSettings, ids []string) (DoctorJob, error) {
	u, e := actor(ctx)
	if e != nil {
		return DoctorJob{}, e
	}
	if !u.IsAdmin {
		return DoctorJob{}, ErrDenied
	}
	if mode != "metadata" && mode != "deep" {
		return DoctorJob{}, ErrInvalid
	}
	tracks := []DoctorTrack{}
	if mode == "metadata" {
		seenLibraries := map[int]bool{}
		for _, library := range settings.LibraryIDs {
			if library < 1 || seenLibraries[library] {
				return DoctorJob{}, ErrInvalid
			}
			seenLibraries[library] = true
		}
		if settings.MaxRows < 1 || settings.MaxRows > 10000 || len(settings.LibraryIDs) > 20 || settings.AfterID != "" && !opaqueID.MatchString(settings.AfterID) {
			return DoctorJob{}, ErrInvalid
		}
		settings = DoctorSettings{MaxRows: settings.MaxRows, LibraryIDs: settings.LibraryIDs, AfterID: settings.AfterID}
	} else {
		if !settings.Confirmed || len(ids) < 1 || len(ids) > 100 || len(settings.SourceBudgets) < 1 || len(settings.SourceBudgets) > 20 || settings.DecodeSeconds < 10 || settings.DecodeSeconds > 600 {
			return DoctorJob{}, ErrInvalid
		}
		settings.MaxRows = 0
		settings.AfterID = ""
		settings.LibraryIDs = nil
		var total int64
		for key, n := range settings.SourceBudgets {
			if key < 1 || n < 1 || n > DoctorMaxSourceBytes {
				return DoctorJob{}, ErrInvalid
			}
			total += n
		}
		if total > DoctorMaxTotalBytes {
			return DoctorJob{}, ErrQuota
		}
		seen := map[string]bool{}
		sizes := map[int]int64{}
		for _, key := range ids {
			if seen[key] {
				return DoctorJob{}, ErrInvalid
			}
			seen[key] = true
			track, e := d.Track(ctx, key)
			if e != nil {
				return DoctorJob{}, ErrDenied
			}
			if track.Size <= 0 || track.Size > DoctorMaxSourceBytes {
				return DoctorJob{}, ErrInvalid
			}
			if settings.SourceBudgets[track.LibraryID] == 0 {
				return DoctorJob{}, ErrInvalid
			}
			sizes[track.LibraryID] += track.Size
			if sizes[track.LibraryID] > settings.SourceBudgets[track.LibraryID] {
				return DoctorJob{}, ErrQuota
			}
			tracks = append(tracks, track)
		}
	}
	now := d.now().Format(time.RFC3339Nano)
	key := id.NewRandom()
	b, e := json.Marshal(settings)
	if e != nil {
		return DoctorJob{}, e
	}
	tx, e := d.DB.BeginTx(ctx, nil)
	if e != nil {
		return DoctorJob{}, e
	}
	defer tx.Rollback()
	var count, items int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_doctor_job`).Scan(&count); e != nil {
		return DoctorJob{}, e
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_doctor_item`).Scan(&items); e != nil {
		return DoctorJob{}, e
	}
	if count >= 100 || items+len(tracks) > DoctorMaxItems {
		return DoctorJob{}, ErrQuota
	}
	var active int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_doctor_job WHERE state IN ('queued','active')`).Scan(&active); e != nil {
		return DoctorJob{}, e
	}
	if active > 0 {
		return DoctorJob{}, ErrConflict
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO family_doctor_job(id,owner_id,mode,state,settings,cursor,created_at,updated_at) VALUES(?,?,?,'queued',?,?,?,?)`, key, u.ID, mode, string(b), settings.AfterID, now, now); e != nil {
		return DoctorJob{}, e
	}
	for library, limit := range settings.SourceBudgets {
		if _, e = tx.ExecContext(ctx, `INSERT INTO family_doctor_budget(job_id,library_id,limit_bytes) VALUES(?,?,?)`, key, library, limit); e != nil {
			return DoctorJob{}, e
		}
	}
	for _, track := range tracks {
		raw, e := json.Marshal(track)
		if e != nil {
			return DoctorJob{}, e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO family_doctor_item(job_id,track_id,library_id,snapshot,state,updated_at) VALUES(?,?,?,?,'queued',?)`, key, track.ID, track.LibraryID, string(raw), now); e != nil {
			return DoctorJob{}, e
		}
	}
	if e = tx.Commit(); e != nil {
		return DoctorJob{}, e
	}
	return d.Get(ctx, key)
}
func (d Doctor) Control(ctx context.Context, key, action string, retry bool) (DoctorJob, error) {
	x, e := d.authorize(ctx, key)
	if e != nil {
		return x, e
	}
	state := ""
	switch action {
	case "pause":
		state = "paused"
	case "cancel":
		state = "cancelled"
	case "resume":
		state = "active"
	default:
		return x, ErrInvalid
	}
	if x.State == "cancelled" || x.State == "failed" {
		return x, ErrConflict
	}
	if x.State == "completed" && !retry {
		return x, ErrConflict
	}
	tx, e := d.DB.BeginTx(ctx, nil)
	if e != nil {
		return x, e
	}
	defer tx.Rollback()
	var currentState string
	if e = tx.QueryRowContext(ctx, `SELECT state FROM family_doctor_job WHERE id=?`, key).Scan(&currentState); e != nil {
		return x, e
	}
	if currentState == "cancelled" || currentState == "failed" || currentState == "completed" && !retry {
		return x, ErrConflict
	}
	if state == "active" {
		var active int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_doctor_job WHERE state IN ('queued','active') AND id<>?`, key).Scan(&active); e != nil {
			return x, e
		}
		if active > 0 {
			return x, ErrConflict
		}
	}
	now := d.now().Format(time.RFC3339Nano)
	if _, e = tx.ExecContext(ctx, `UPDATE family_doctor_job SET state=?,updated_at=? WHERE id=?`, state, now, key); e != nil {
		return x, e
	}
	if state == "cancelled" {
		if _, e = tx.ExecContext(ctx, `UPDATE family_doctor_item SET state='cancelled',lease='',lease_until=0,updated_at=? WHERE job_id=? AND state IN ('queued','leased','fetching','downloaded','budget')`, now, key); e != nil {
			return x, e
		}
	}
	if state == "paused" {
		if _, e = tx.ExecContext(ctx, `UPDATE family_doctor_item SET state='queued',lease='',lease_until=0,updated_at=? WHERE job_id=? AND state IN ('leased','fetching','downloaded')`, now, key); e != nil {
			return x, e
		}
	}
	if state == "active" && retry {
		if _, e = tx.ExecContext(ctx, `UPDATE family_doctor_item SET state='queued',lease='',lease_until=0,updated_at=? WHERE job_id=? AND state IN ('failed','unavailable','budget') AND attempts<3`, now, key); e != nil {
			return x, e
		}
	}
	if e = tx.Commit(); e != nil {
		return x, e
	}
	return d.Get(ctx, key)
}
func (d Doctor) RaiseBudgets(ctx context.Context, key string, limits map[int]int64) (DoctorJob, error) {
	x, e := d.authorize(ctx, key)
	if e != nil {
		return x, e
	}
	if x.Mode != "deep" || x.State == "cancelled" || len(limits) < 1 || len(limits) > 20 {
		return x, ErrInvalid
	}
	tx, e := d.DB.BeginTx(ctx, nil)
	if e != nil {
		return x, e
	}
	defer tx.Rollback()
	var total int64
	rows, e := tx.QueryContext(ctx, `SELECT library_id,limit_bytes FROM family_doctor_budget WHERE job_id=?`, key)
	if e != nil {
		return x, e
	}
	current := map[int]int64{}
	for rows.Next() {
		var lib int
		var n int64
		if e = rows.Scan(&lib, &n); e != nil {
			rows.Close()
			return x, e
		}
		current[lib] = n
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return x, e
	}
	for lib, n := range limits {
		old, ok := current[lib]
		if !ok || n < old || n > DoctorMaxSourceBytes {
			return x, ErrInvalid
		}
		current[lib] = n
	}
	for _, n := range current {
		total += n
	}
	if total > DoctorMaxTotalBytes {
		return x, ErrQuota
	}
	for lib, n := range current {
		if _, e = tx.ExecContext(ctx, `UPDATE family_doctor_budget SET limit_bytes=? WHERE job_id=? AND library_id=?`, n, key, lib); e != nil {
			return x, e
		}
	}
	if e = tx.Commit(); e != nil {
		return x, e
	}
	return d.Get(ctx, key)
}
func (d Doctor) Items(ctx context.Context, key string, offset int) ([]DoctorItem, error) {
	if _, e := d.authorize(ctx, key); e != nil {
		return nil, e
	}
	if offset < 0 || offset > 100000 {
		return nil, ErrInvalid
	}
	rows, e := d.DB.QueryContext(ctx, `SELECT snapshot,state,attempts,bytes_read,sha256,result FROM family_doctor_item WHERE job_id=? ORDER BY track_id LIMIT 50 OFFSET ?`, key, offset)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []DoctorItem{}
	for rows.Next() {
		var x DoctorItem
		var snap, result string
		if e = rows.Scan(&snap, &x.State, &x.Attempts, &x.BytesRead, &x.SHA256, &result); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(snap), &x.Track); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(result), &x.Result); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (d Doctor) Claim(ctx context.Context, key string) (*DoctorItem, error) {
	x, e := d.authorize(ctx, key)
	if e != nil {
		return nil, e
	}
	if x.Mode != "deep" || x.State != "queued" && x.State != "active" {
		return nil, ErrConflict
	}
	tx, e := d.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	now := d.now()
	var currentState string
	if e = tx.QueryRowContext(ctx, `SELECT state FROM family_doctor_job WHERE id=?`, key).Scan(&currentState); e != nil {
		return nil, e
	}
	if currentState != "queued" && currentState != "active" {
		return nil, ErrConflict
	}
	// A crashed or sleeping companion can be resumed; each fresh attempt reserves
	// another full payload, so retries cannot silently exceed a source budget.
	if _, e = tx.ExecContext(ctx, `UPDATE family_doctor_item SET state=CASE WHEN attempts<3 THEN 'queued' ELSE 'failed' END,lease='',lease_until=0 WHERE job_id=? AND state IN ('leased','fetching','downloaded') AND lease_until<?`, key, now.UnixMilli()); e != nil {
		return nil, e
	}
	var inFlight int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_doctor_item WHERE job_id=? AND state IN ('leased','fetching','downloaded')`, key).Scan(&inFlight); e != nil {
		return nil, e
	}
	if inFlight > 0 {
		if e = tx.Commit(); e != nil {
			return nil, e
		}
		return nil, nil
	}
	var item DoctorItem
	var snap string
	e = tx.QueryRowContext(ctx, `SELECT snapshot,attempts FROM family_doctor_item WHERE job_id=? AND state='queued' ORDER BY track_id LIMIT 1`, key).Scan(&snap, &item.Attempts)
	if errors.Is(e, sql.ErrNoRows) {
		var outstanding int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_doctor_item WHERE job_id=? AND state IN ('leased','fetching','downloaded')`, key).Scan(&outstanding); e != nil {
			return nil, e
		}
		if outstanding == 0 {
			_, e = tx.ExecContext(ctx, `UPDATE family_doctor_job SET state='completed',updated_at=? WHERE id=?`, now.Format(time.RFC3339Nano), key)
			if e != nil {
				return nil, e
			}
		}
		if e = tx.Commit(); e != nil {
			return nil, e
		}
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal([]byte(snap), &item.Track); e != nil {
		return nil, e
	}
	var limit, reserved int64
	if e = tx.QueryRowContext(ctx, `SELECT limit_bytes,reserved_bytes FROM family_doctor_budget WHERE job_id=? AND library_id=?`, key, item.Track.LibraryID).Scan(&limit, &reserved); e != nil {
		return nil, e
	}
	if item.Track.Size > limit-reserved {
		if _, e = tx.ExecContext(ctx, `UPDATE family_doctor_item SET state='budget',updated_at=? WHERE job_id=? AND track_id=?`, now.Format(time.RFC3339Nano), key, item.Track.ID); e != nil {
			return nil, e
		}
		if e = tx.Commit(); e != nil {
			return nil, e
		}
		return &DoctorItem{Track: item.Track, State: "budget"}, nil
	}
	item.State = "leased"
	item.Lease = id.NewRandom()
	item.LeaseUntil = now.Add(20 * time.Minute).UnixMilli()
	item.Attempts++
	if _, e = tx.ExecContext(ctx, `UPDATE family_doctor_budget SET reserved_bytes=reserved_bytes+? WHERE job_id=? AND library_id=?`, item.Track.Size, key, item.Track.LibraryID); e != nil {
		return nil, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE family_doctor_item SET state='leased',lease=?,lease_until=?,attempts=attempts+1,bytes_read=0,sha256='',result='{}',updated_at=? WHERE job_id=? AND track_id=?`, item.Lease, item.LeaseUntil, now.Format(time.RFC3339Nano), key, item.Track.ID); e != nil {
		return nil, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE family_doctor_job SET state='active',updated_at=? WHERE id=?`, now.Format(time.RFC3339Nano), key); e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return &item, nil
}

// RemoveReport only removes a finished diagnostic run. Original catalog/music
// and the independently stored last successful track evidence are untouched.
func (d Doctor) RemoveReport(ctx context.Context, key string, confirmed bool) error {
	if _, e := d.authorize(ctx, key); e != nil {
		return e
	}
	if !confirmed {
		return ErrInvalid
	}
	res, e := d.DB.ExecContext(ctx, `DELETE FROM family_doctor_job WHERE id=? AND state IN ('completed','cancelled')`, key)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return ErrConflict
	}
	return nil
}
