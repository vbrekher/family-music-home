package family

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var requestNext = map[string][]string{
	"requested":   {"reviewing", "declined", "unavailable", "cancelled"},
	"reviewing":   {"looking", "obtained", "added", "declined", "unavailable", "cancelled"},
	"looking":     {"obtained", "added", "declined", "unavailable", "cancelled"},
	"obtained":    {"added", "unavailable", "cancelled"},
	"unavailable": {"reviewing", "cancelled"},
	"declined":    {"reviewing"},
}

func (s Store) SubmitRequest(ctx context.Context, key string, revision int, owners []string) (Entry, error) {
	u, e := actor(ctx)
	if e != nil {
		return Entry{}, e
	}
	x, e := s.GetKind(ctx, "request", key)
	if e != nil {
		return x, e
	}
	if x.OwnerID != u.ID {
		return x, ErrDenied
	}
	if x.Body.State == "requested" {
		return x, nil
	}
	if x.Body.State != "draft" || x.Revision != revision || len(owners) == 0 {
		return x, ErrConflict
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return x, e
	}
	defer tx.Rollback()
	var daily int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_action a JOIN family_entry e ON e.id=a.entry_id WHERE e.owner_id=? AND a.action_key='submit' AND a.created_at>=?`, u.ID, time.Now().UTC().Format("2006-01-02")).Scan(&daily); e != nil {
		return x, e
	}
	if daily >= 100 {
		return x, ErrQuota
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO family_action(entry_id,action_key,created_at) VALUES(?,'submit',?)`, key, time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
		return x, ErrConflict
	}
	res, e := tx.ExecContext(ctx, `UPDATE family_entry SET body=json_set(body,'$.state','requested'),revision=revision+1,updated_at=? WHERE id=? AND owner_id=? AND revision=?`, time.Now().UTC().Format(time.RFC3339Nano), key, u.ID, revision)
	if e != nil {
		return x, e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return x, ErrConflict
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM family_access WHERE entry_id=?`, key); e != nil {
		return x, e
	}
	for _, owner := range owners {
		if s.UserExists == nil || !s.UserExists(ctx, owner) || s.IsAdministrator == nil || !s.IsAdministrator(ctx, owner) {
			return x, ErrInvalid
		}
		if owner == u.ID {
			continue
		}
		if _, e = tx.ExecContext(ctx, `INSERT OR IGNORE INTO family_access(entry_id,user_id) VALUES(?,?)`, key, owner); e != nil {
			return x, e
		}
		if e = notify(ctx, tx, owner, key, "A family music request needs review", time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
			return x, e
		}
	}
	for _, item := range x.Body.Items {
		if _, e = tx.ExecContext(ctx, `INSERT INTO family_request_history(entry_id,item_id,actor_id,state,comment,created_at) VALUES(?,?,?,?,?,?)`, key, item.ID, u.ID, "requested", "Request submitted", time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
			return x, e
		}
	}
	if e = tx.Commit(); e != nil {
		return x, e
	}
	return s.Get(ctx, key)
}
func (s Store) UpdateRequest(ctx context.Context, key, itemID, state, comment string, revision int, resolution *Reference) (Entry, error) {
	u, e := actor(ctx)
	if e != nil {
		return Entry{}, e
	}
	x, e := s.GetKind(ctx, "request", key)
	if e != nil {
		return x, e
	}
	if x.Body.State == "draft" || x.Revision != revision || len(comment) > 4000 {
		return x, ErrConflict
	}
	if !u.IsAdmin && (x.OwnerID != u.ID || state != "cancelled") {
		return x, ErrDenied
	}
	var raw string
	if e = s.DB.QueryRowContext(ctx, `SELECT body FROM family_entry WHERE id=? AND revision=?`, key, revision).Scan(&raw); errors.Is(e, sql.ErrNoRows) {
		return x, ErrConflict
	} else if e != nil {
		return x, e
	}
	rawBody, e := decodeStored(raw)
	if e != nil {
		return x, e
	}
	x.Body = rawBody
	found := false
	for i, item := range x.Body.Items {
		if item.ID != itemID {
			continue
		}
		found = true
		allowed := false
		for _, v := range requestNext[item.State] {
			if v == state {
				allowed = true
			}
		}
		if !allowed {
			return x, ErrInvalid
		}
		if resolution != nil {
			if resolution.Kind != "track" && resolution.Kind != "album" {
				return x, ErrInvalid
			}
			if !u.IsAdmin || state != "added" && state != "obtained" {
				return x, ErrDenied
			}
			v, e := s.Resolve(ctx, resolution.Kind, resolution.ID)
			if e != nil {
				return x, ErrDenied
			}
			resolution = &v
		}
		if state == "added" && resolution == nil && item.Resolution != nil {
			if _, e := s.Resolve(ctx, item.Resolution.Kind, item.Resolution.ID); e != nil {
				return x, ErrDenied
			}
		}
		if state == "added" && resolution == nil && item.Resolution == nil {
			return x, ErrInvalid
		}
		x.Body.Items[i].State = state
		if resolution != nil {
			x.Body.Items[i].Resolution = resolution
		}
	}
	if !found {
		return x, ErrInvalid
	}
	x.Body.State = "in-progress"
	allAdded, allClosed := true, true
	for _, item := range x.Body.Items {
		allAdded = allAdded && item.State == "added"
		allClosed = allClosed && (item.State == "added" || item.State == "cancelled" || item.State == "declined" || item.State == "unavailable")
	}
	if allAdded {
		x.Body.State = "added"
	} else if allClosed {
		x.Body.State = "closed"
	}
	b, e := json.Marshal(x.Body)
	if e != nil {
		return x, e
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return x, e
	}
	defer tx.Rollback()
	var histories int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_request_history`).Scan(&histories); e != nil {
		return x, e
	}
	if histories >= 100000 {
		return x, ErrQuota
	}
	res, e := tx.ExecContext(ctx, `UPDATE family_entry SET body=?,revision=revision+1,updated_at=? WHERE id=? AND revision=?`, string(b), now, key, revision)
	if e != nil {
		return x, e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return x, ErrConflict
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO family_request_history(entry_id,item_id,actor_id,state,comment,created_at) VALUES(?,?,?,?,?,?)`, key, itemID, u.ID, state, strings.TrimSpace(comment), now); e != nil {
		return x, e
	}
	if u.ID != x.OwnerID {
		if e = notify(ctx, tx, x.OwnerID, key, "Your music request has an update", now); e != nil {
			return x, e
		}
	}
	if e = tx.Commit(); e != nil {
		return x, e
	}
	return s.Get(ctx, key)
}
func (s Store) RequestHistory(ctx context.Context, key string, offsets ...int) ([]map[string]string, error) {
	if _, e := s.GetKind(ctx, "request", key); e != nil {
		return nil, e
	}
	offset := 0
	if len(offsets) > 0 {
		offset = offsets[0]
	}
	if offset < 0 || offset > 10000 {
		return nil, ErrInvalid
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT item_id,actor_id,state,comment,created_at FROM family_request_history WHERE entry_id=? ORDER BY id DESC LIMIT 100 OFFSET ?`, key, offset)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var item, actor, state, comment, at string
		if e = rows.Scan(&item, &actor, &state, &comment, &at); e != nil {
			return nil, e
		}
		out = append(out, map[string]string{"itemId": item, "actorId": actor, "state": state, "comment": comment, "createdAt": at})
	}
	return out, rows.Err()
}
