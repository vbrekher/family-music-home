package family

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"
)

var actionKey = regexp.MustCompile(`^[A-Za-z0-9_-]{16,80}$`)

func NextAnniversary(now time.Time, day, clock, zone string) (time.Time, error) {
	loc, e := time.LoadLocation(zone)
	if e != nil {
		return time.Time{}, ErrInvalid
	}
	d, e := time.Parse("01-02", day)
	if e != nil {
		return time.Time{}, ErrInvalid
	}
	h, e := time.Parse("15:04", clock)
	if e != nil {
		return time.Time{}, ErrInvalid
	}
	local := now.In(loc)
	for year := local.Year(); year <= local.Year()+8; year++ {
		v := time.Date(year, d.Month(), d.Day(), h.Hour(), h.Minute(), 0, 0, loc)
		if v.Month() != d.Month() || v.Day() != d.Day() {
			continue
		}
		if v.Hour() != h.Hour() || v.Minute() != h.Minute() {
			return time.Time{}, ErrInvalid
		}
		if v.After(now) {
			return v.UTC(), nil
		}
	}
	return time.Time{}, ErrInvalid
}
func (s Store) SendLetter(ctx context.Context, key string, revision int, sendKey string, resend bool) (Entry, error) {
	u, e := actor(ctx)
	if e != nil {
		return Entry{}, e
	}
	if !actionKey.MatchString(sendKey) {
		return Entry{}, ErrInvalid
	}
	x, e := s.GetKind(ctx, "letter", key)
	if e != nil || x.OwnerID != u.ID {
		if e != nil {
			return Entry{}, e
		}
		return Entry{}, ErrDenied
	}
	var exists int
	e = s.DB.QueryRowContext(ctx, `SELECT count(*) FROM family_action WHERE entry_id=? AND action_key=?`, key, "send:"+sendKey).Scan(&exists)
	if e != nil {
		return x, e
	}
	if exists > 0 {
		if e = s.DeliverDue(ctx, time.Now()); e != nil {
			return x, e
		}
		return s.Get(ctx, key)
	}
	if x.Revision != revision || x.Body.State != "draft" && !(x.Body.State == "sent" && resend) {
		return x, ErrConflict
	}
	if len(x.SharedWith) < 1 || len(x.Body.References) == 0 && strings.TrimSpace(SanitizeMessage(x.Body.Message)) == "" {
		return x, ErrInvalid
	}
	now := time.Now().UTC()
	due := now
	if !resend && x.Body.DeliveryAt != "" {
		due, e = time.Parse(time.RFC3339, x.Body.DeliveryAt)
		if e != nil || due.Before(now.Add(-time.Second)) {
			return x, ErrInvalid
		}
	}
	if !resend && x.Body.DeliveryAt == "" && x.Body.Anniversary != "" {
		due, e = NextAnniversary(now, x.Body.Anniversary, x.Body.AnniversaryTime, x.Body.Timezone)
		if e != nil {
			return x, e
		}
	}
	for _, r := range x.Body.References {
		if _, e = s.Resolve(ctx, r.Kind, r.ID); e != nil {
			return x, ErrDenied
		}
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return x, e
	}
	defer tx.Rollback()
	var pending, sends, deliveries, daily int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_attachment WHERE entry_id=? AND state='pending'`, key).Scan(&pending); e != nil {
		return x, e
	}
	if pending > 0 {
		return x, ErrConflict
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_action WHERE entry_id=? AND action_key LIKE 'send:%'`, key).Scan(&sends); e != nil {
		return x, e
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_delivery`).Scan(&deliveries); e != nil {
		return x, e
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_action a JOIN family_entry e ON e.id=a.entry_id WHERE e.owner_id=? AND a.action_key LIKE 'send:%' AND a.created_at>=?`, u.ID, now.Format("2006-01-02")).Scan(&daily); e != nil {
		return x, e
	}
	if sends >= 100 || deliveries >= 100000 || daily >= 100 {
		return x, ErrQuota
	}
	result, e := tx.ExecContext(ctx, `UPDATE family_entry SET body=json_set(body,'$.state','scheduled'),revision=revision+1,updated_at=? WHERE id=? AND owner_id=? AND revision=?`, now.Format(time.RFC3339Nano), key, u.ID, revision)
	if e != nil {
		return x, e
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return x, ErrConflict
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO family_action(entry_id,action_key,created_at) VALUES(?,?,?)`, key, "send:"+sendKey, now.Format(time.RFC3339Nano)); e != nil {
		return x, ErrConflict
	}
	for _, recipient := range x.SharedWith {
		if s.UserExists == nil || !s.UserExists(ctx, recipient) {
			return x, ErrInvalid
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO family_delivery(entry_id,recipient_id,due_at,send_key,recurring) VALUES(?,?,?,?,?)`, key, recipient, due.UnixMilli(), sendKey, !resend && x.Body.Anniversary != ""); e != nil {
			return x, e
		}
	}
	if e = tx.Commit(); e != nil {
		return x, e
	}
	if e = s.DeliverDue(ctx, now); e != nil {
		return x, e
	}
	return s.Get(ctx, key)
}
func (s Store) CancelLetter(ctx context.Context, key string, revision int) (Entry, error) {
	u, e := actor(ctx)
	if e != nil {
		return Entry{}, e
	}
	x, e := s.GetKind(ctx, "letter", key)
	if e != nil {
		return x, e
	}
	if x.OwnerID != u.ID {
		return x, ErrDenied
	}
	if x.Revision != revision {
		return x, ErrConflict
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return x, e
	}
	defer tx.Rollback()
	var delivered int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_delivery WHERE entry_id=? AND delivered_at IS NOT NULL`, key).Scan(&delivered); e != nil {
		return x, e
	}
	state := "draft"
	if delivered > 0 {
		state = "sent"
	}
	res, e := tx.ExecContext(ctx, `UPDATE family_entry SET body=json_set(body,'$.state',?,'$.anniversary',''),revision=revision+1,updated_at=? WHERE id=? AND owner_id=? AND revision=?`, state, time.Now().UTC().Format(time.RFC3339Nano), key, u.ID, revision)
	if e != nil {
		return x, e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return x, ErrConflict
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM family_delivery WHERE entry_id=? AND delivered_at IS NULL`, key); e != nil {
		return x, e
	}
	if e = tx.Commit(); e != nil {
		return x, e
	}
	return s.Get(ctx, key)
}
func (s Store) MarkLetterRead(ctx context.Context, key string) error {
	u, e := actor(ctx)
	if e != nil {
		return e
	}
	if _, e = s.GetKind(ctx, "letter", key); e != nil {
		return e
	}
	_, e = s.DB.ExecContext(ctx, `UPDATE family_delivery SET read_at=coalesce(read_at,?) WHERE entry_id=? AND recipient_id=? AND delivered_at IS NOT NULL`, time.Now().UTC().UnixMilli(), key, u.ID)
	return e
}

// DeliverDue is called by the existing scheduler and once at startup. The whole
// inbox transition is transactional; a second scheduler cannot deliver it twice.
func (s Store) DeliverDue(ctx context.Context, now time.Time) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT d.entry_id,d.recipient_id,d.due_at,d.send_key,e.body,d.recurring FROM family_delivery d JOIN family_entry e ON e.id=d.entry_id WHERE d.delivered_at IS NULL AND d.due_at<=? AND json_extract(e.body,'$.state') IN ('scheduled','sent') ORDER BY d.due_at LIMIT 50`, now.UnixMilli())
	if e != nil {
		return e
	}
	type due struct {
		recurring               bool
		entry, user, send, body string
		at                      int64
	}
	work := []due{}
	for rows.Next() {
		var d due
		if e = rows.Scan(&d.entry, &d.user, &d.at, &d.send, &d.body, &d.recurring); e != nil {
			rows.Close()
			return e
		}
		work = append(work, d)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, d := range work {
		res, e := tx.ExecContext(ctx, `UPDATE family_delivery SET delivered_at=? WHERE entry_id=? AND recipient_id=? AND due_at=? AND send_key=? AND delivered_at IS NULL`, now.UnixMilli(), d.entry, d.user, d.at, d.send)
		if e != nil {
			return e
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			continue
		}
		if _, e = tx.ExecContext(ctx, `UPDATE family_entry SET body=json_set(body,'$.state','sent'),revision=revision+1,updated_at=? WHERE id=? AND json_extract(body,'$.state')='scheduled'`, now.UTC().Format(time.RFC3339Nano), d.entry); e != nil {
			return e
		}
		if e = notify(ctx, tx, d.user, d.entry, "A music letter is in your inbox", now.UTC().Format(time.RFC3339Nano)); e != nil {
			return e
		}
		body, e := decodeStored(d.body)
		if e != nil {
			return e
		}
		if body.Anniversary != "" && d.recurring {
			var deliveries int
			if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_delivery`).Scan(&deliveries); e != nil {
				return e
			}
			if deliveries >= 100000 {
				continue
			}
			next, e := NextAnniversary(now.Add(time.Minute), body.Anniversary, body.AnniversaryTime, body.Timezone)
			if e != nil {
				return e
			}
			if _, e = tx.ExecContext(ctx, `INSERT OR IGNORE INTO family_delivery(entry_id,recipient_id,due_at,send_key,recurring) VALUES(?,?,?,?,1)`, d.entry, d.user, next.UnixMilli(), d.send); e != nil {
				return e
			}
		}
	}
	return tx.Commit()
}
func (s Store) Notifications(ctx context.Context) ([]map[string]any, error) {
	u, e := actor(ctx)
	if e != nil {
		return nil, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT n.id,n.entry_id,n.message,n.created_at,n.read_at,e.kind FROM family_notification n JOIN family_entry e ON e.id=n.entry_id WHERE n.user_id=? AND `+readAccess+` ORDER BY n.created_at DESC LIMIT 50`, u.ID, u.ID, u.ID, u.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, entry, msg, at, kind string
		var read sql.NullString
		if e = rows.Scan(&id, &entry, &msg, &at, &read, &kind); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "entryId": entry, "kind": kind, "message": msg, "createdAt": at, "read": read.Valid})
	}
	return out, rows.Err()
}
func (s Store) ReadNotification(ctx context.Context, key string) error {
	u, e := actor(ctx)
	if e != nil {
		return e
	}
	_, e = s.DB.ExecContext(ctx, `UPDATE family_notification SET read_at=coalesce(read_at,?) WHERE id=? AND user_id=?`, time.Now().UTC().Format(time.RFC3339Nano), key, u.ID)
	return e
}
