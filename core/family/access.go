package family

import (
	"context"
	"database/sql"
	"errors"
)

// Access never derives a reader grant from administrator status. Draft capsules
// and letters stay private even when their intended recipients have been saved.
const readAccess = `(e.owner_id=? OR (EXISTS(SELECT 1 FROM family_access a WHERE a.entry_id=e.id AND a.user_id=?) AND (e.kind='shelf' OR (e.kind='capsule' AND json_extract(e.body,'$.state')='published') OR (e.kind='request' AND json_extract(e.body,'$.state')<>'draft') OR (e.kind='letter' AND EXISTS(SELECT 1 FROM family_delivery d WHERE d.entry_id=e.id AND d.recipient_id=? AND d.delivered_at IS NOT NULL)))))`

func (s Store) CanRead(ctx context.Context, key string) error {
	u, e := actor(ctx)
	if e != nil {
		return e
	}
	var owner string
	e = s.DB.QueryRowContext(ctx, `SELECT e.owner_id FROM family_entry e WHERE id=? AND `+readAccess, key, u.ID, u.ID, u.ID).Scan(&owner)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrDenied
	}
	if e == nil && !ChildReaderAllowed(u, owner) {
		return ErrDenied
	}
	return e
}
func (s Store) GetKind(ctx context.Context, kind, key string) (Entry, error) {
	x, e := s.Get(ctx, key)
	if e == nil && x.Kind != kind {
		return Entry{}, ErrDenied
	}
	return x, e
}
func (s Store) ArchiveInbox(ctx context.Context, key string, archived bool) error {
	u, e := actor(ctx)
	if e != nil {
		return e
	}
	x, e := s.Get(ctx, key)
	if e != nil {
		return e
	}
	if x.Kind != "letter" && x.Kind != "request" {
		return ErrInvalid
	}
	_, e = s.DB.ExecContext(ctx, `INSERT INTO family_inbox_state(entry_id,user_id,archived) VALUES(?,?,?) ON CONFLICT(entry_id,user_id) DO UPDATE SET archived=excluded.archived`, key, u.ID, archived)
	return e
}
