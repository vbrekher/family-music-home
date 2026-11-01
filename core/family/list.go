package family

import (
	"context"
	"strings"
	"time"
)

type ListOptions struct {
	Archived              bool
	Offset                int
	View, Query, From, To string
}

func (s Store) ListEntries(ctx context.Context, kind string, opt ListOptions) ([]Entry, error) {
	u, e := actor(ctx)
	if e != nil {
		return nil, e
	}
	if !validKind(kind) || opt.Offset < 0 || opt.Offset > 10000 || len(opt.Query) > 100 {
		return nil, ErrInvalid
	}
	query := `SELECT id FROM family_entry e WHERE kind=? AND (CASE WHEN e.kind IN ('letter','request') THEN coalesce((SELECT archived FROM family_inbox_state fi WHERE fi.entry_id=e.id AND fi.user_id=?),0) ELSE e.archived END)=? AND ` + readAccess
	args := []any{kind, u.ID, opt.Archived, u.ID, u.ID, u.ID}
	switch opt.View {
	case "", "all":
	case "sent":
		query += ` AND owner_id=? AND json_extract(body,'$.state') IN ('sent','scheduled')`
		args = append(args, u.ID)
	case "unread":
		if kind != "letter" {
			return nil, ErrInvalid
		}
		query += ` AND EXISTS (SELECT 1 FROM family_delivery d WHERE d.entry_id=e.id AND d.recipient_id=? AND d.delivered_at IS NOT NULL AND d.read_at IS NULL)`
		args = append(args, u.ID)
	case "mine":
		query += ` AND owner_id=?`
		args = append(args, u.ID)
	case "inbox":
		query += ` AND owner_id<>?`
		args = append(args, u.ID)
	case "draft":
		query += ` AND owner_id=? AND json_extract(body,'$.state')='draft'`
		args = append(args, u.ID)
	default:
		return nil, ErrInvalid
	}
	if q := strings.TrimSpace(opt.Query); q != "" {
		query += ` AND (title LIKE ? OR json_extract(body,'$.description') LIKE ?)`
		args = append(args, "%"+q+"%", "%"+q+"%")
	}
	if opt.From != "" {
		if _, e = time.Parse("2006-01-02", opt.From); e != nil {
			return nil, ErrInvalid
		}
		query += ` AND coalesce(nullif(json_extract(body,'$.dateTo'),''),json_extract(body,'$.dateFrom'))>=?`
		args = append(args, opt.From)
	}
	if opt.To != "" {
		if _, e = time.Parse("2006-01-02", opt.To); e != nil {
			return nil, ErrInvalid
		}
		query += ` AND json_extract(body,'$.dateFrom')<=?`
		args = append(args, opt.To)
	}
	if kind == "capsule" {
		query += ` ORDER BY json_extract(body,'$.dateFrom') DESC,updated_at DESC,id LIMIT 25 OFFSET ?`
	} else {
		query += ` ORDER BY updated_at DESC,id LIMIT 25 OFFSET ?`
	}
	if u.FamilyChild != nil && !u.FamilyChild.AllowFamilyMessages {
		idx := strings.Index(query, " ORDER BY")
		query = query[:idx] + " AND e.owner_id IN (?,?)" + query[idx:]
		args = append(args, u.ID, u.FamilyChild.ParentID)
	}
	args = append(args, opt.Offset)
	rows, e := s.DB.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []Entry{}
	for _, id := range ids {
		x, e := s.Get(ctx, id)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, nil
}
