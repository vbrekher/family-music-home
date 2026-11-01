// Package family stores private family curation. Catalog identities and playback
// remain Navidrome's responsibility; these records never contain audio or paths.
package family

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/id"
	"github.com/navidrome/navidrome/model/request"
)

var ErrInvalid = errors.New("invalid family content")
var ErrConflict = errors.New("content changed; reload before saving")
var ErrDenied = errors.New("content unavailable")

type Reference struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Title     string `json:"title"`
	Artist    string `json:"artist,omitempty"`
	Available bool   `json:"available"`
}
type Content struct {
	CoverAttachmentID string        `json:"coverAttachmentId,omitempty"`
	State             string        `json:"state,omitempty"`
	Message           string        `json:"message,omitempty"`
	DateFrom          string        `json:"dateFrom,omitempty"`
	DateTo            string        `json:"dateTo,omitempty"`
	Timezone          string        `json:"timezone,omitempty"`
	DeliveryAt        string        `json:"deliveryAt,omitempty"`
	Anniversary       string        `json:"anniversary,omitempty"`
	AnniversaryTime   string        `json:"anniversaryTime,omitempty"`
	Items             []RequestItem `json:"items,omitempty"`
	Description       string        `json:"description"`
	References        []Reference   `json:"references"`
	Hidden            bool          `json:"hidden"`
	Cover             *Reference    `json:"cover,omitempty"`
}
type Entry struct {
	Deliveries []Delivery `json:"deliveries,omitempty"`
	Unread     bool       `json:"unread"`
	CanEdit    bool       `json:"canEdit"`
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	OwnerID    string     `json:"ownerId"`
	Title      string     `json:"title"`
	Body       Content    `json:"body"`
	SharedWith []string   `json:"sharedWith"`
	Archived   bool       `json:"archived"`
	Revision   int        `json:"revision"`
	CreatedAt  string     `json:"createdAt"`
	UpdatedAt  string     `json:"updatedAt"`
}
type Resolver func(context.Context, string, string) (Reference, error)
type Store struct {
	MediaRoot       string
	ProbePath       string
	DB              *sql.DB
	Resolve         Resolver
	UserExists      func(context.Context, string) bool
	IsAdministrator func(context.Context, string) bool
}

func actor(ctx context.Context) (model.User, error) {
	u, ok := request.UserFrom(ctx)
	if !ok || u.ID == "" {
		return u, ErrDenied
	}
	return u, nil
}
func (s Store) List(ctx context.Context, kind string, archived bool, offset int) ([]Entry, error) {
	return s.ListEntries(ctx, kind, ListOptions{Archived: archived, Offset: offset})
}
func (s Store) Get(ctx context.Context, key string) (Entry, error) {
	u, e := actor(ctx)
	if e != nil {
		return Entry{}, e
	}
	var x Entry
	var body string
	e = s.DB.QueryRowContext(ctx, `SELECT id,kind,owner_id,title,body,archived,revision,created_at,updated_at FROM family_entry e WHERE id=? AND `+readAccess, key, u.ID, u.ID, u.ID).Scan(&x.ID, &x.Kind, &x.OwnerID, &x.Title, &body, &x.Archived, &x.Revision, &x.CreatedAt, &x.UpdatedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return x, ErrDenied
	}
	if e != nil {
		return x, e
	}
	if !ChildReaderAllowed(u, x.OwnerID) {
		return x, ErrDenied
	}
	if e = json.Unmarshal([]byte(body), &x.Body); e != nil {
		return x, e
	}
	if x.Body.References == nil {
		x.Body.References = []Reference{}
	}
	x.CanEdit = x.OwnerID == u.ID
	x.SharedWith = []string{}
	if x.OwnerID == u.ID {
		rows, e := s.DB.QueryContext(ctx, `SELECT user_id FROM family_access WHERE entry_id=? ORDER BY user_id`, key)
		if e != nil {
			return x, e
		}
		for rows.Next() {
			var v string
			if e = rows.Scan(&v); e != nil {
				rows.Close()
				return x, e
			}
			x.SharedWith = append(x.SharedWith, v)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return x, e
		}
	}
	// Revalidate each reference on every read. Revoked library access must also
	// redact snapshots; offline sources retain metadata through the catalog.
	for i, r := range x.Body.References {
		v, e := s.Resolve(ctx, r.Kind, r.ID)
		if e != nil {
			x.Body.References[i] = Reference{Kind: r.Kind, Title: "Unavailable item"}
			continue
		}
		x.Body.References[i] = r
		x.Body.References[i].Available = v.Available
	}
	if r := x.Body.Cover; r != nil {
		v, e := s.Resolve(ctx, r.Kind, r.ID)
		if e != nil {
			x.Body.Cover = nil
		} else {
			x.Body.Cover = &v
		}
	}

	for i, item := range x.Body.Items {
		if item.Resolution != nil {
			v, e := s.Resolve(ctx, item.Resolution.Kind, item.Resolution.ID)
			if e != nil {
				x.Body.Items[i].Resolution = nil
			} else {
				v.Title = item.Resolution.Title
				v.Artist = item.Resolution.Artist
				x.Body.Items[i].Resolution = &v
			}
		}
	}
	if x.Kind == "letter" {
		rows, e := s.DB.QueryContext(ctx, `SELECT recipient_id,due_at,delivered_at,read_at FROM family_delivery WHERE entry_id=? AND (? OR recipient_id=?) ORDER BY due_at DESC LIMIT 100`, key, x.OwnerID == u.ID, u.ID)
		if e != nil {
			return x, e
		}
		for rows.Next() {
			var d Delivery
			if e = rows.Scan(&d.RecipientID, &d.DueAt, &d.DeliveredAt, &d.ReadAt); e != nil {
				rows.Close()
				return x, e
			}
			x.Deliveries = append(x.Deliveries, d)
			if d.RecipientID == u.ID && d.DeliveredAt != nil && d.ReadAt == nil {
				x.Unread = true
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return x, e
		}
	}
	if x.Kind == "letter" || x.Kind == "request" {
		var archived int
		e := s.DB.QueryRowContext(ctx, `SELECT archived FROM family_inbox_state WHERE entry_id=? AND user_id=?`, key, u.ID).Scan(&archived)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return x, e
		}
		x.Archived = archived != 0
	}
	if x.Body.CoverAttachmentID != "" {
		var visible int
		e := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM family_attachment WHERE id=? AND entry_id=? AND state='ready' AND hidden=0`, x.Body.CoverAttachmentID, x.ID).Scan(&visible)
		if e != nil {
			return x, e
		}
		if visible == 0 {
			x.Body.CoverAttachmentID = ""
		}
	}
	return x, nil
}
func (s Store) Save(ctx context.Context, x Entry) (Entry, error) {
	u, e := actor(ctx)
	if e != nil {
		return x, e
	}
	x.Title = strings.TrimSpace(x.Title)
	if !validKind(x.Kind) || x.Title == "" || utf8.RuneCountInString(x.Title) > 160 || len(x.Body.Description) > 16000 || len(x.Body.References) > 100 || len(x.SharedWith) > 20 {
		return x, ErrInvalid
	}
	var old *Entry
	if x.ID != "" {
		var storedKind, storedBody string
		var revision int
		err := s.DB.QueryRowContext(ctx, `SELECT kind,body,revision FROM family_entry WHERE id=? AND owner_id=?`, x.ID, u.ID).Scan(&storedKind, &storedBody, &revision)
		if errors.Is(err, sql.ErrNoRows) {
			return x, ErrConflict
		}
		if err != nil {
			return x, err
		}
		if storedKind != x.Kind || revision != x.Revision {
			return x, ErrConflict
		}
		b, err := decodeStored(storedBody)
		if err != nil {
			return x, err
		}
		old = &Entry{Body: b}
	}
	if e = s.prepareContent(ctx, &x, old); e != nil {
		return x, e
	}
	if x.Body.CoverAttachmentID != "" {
		var mime string
		e := s.DB.QueryRowContext(ctx, `SELECT mime FROM family_attachment WHERE id=? AND entry_id=? AND owner_id=? AND state='ready' AND hidden=0`, x.Body.CoverAttachmentID, x.ID, u.ID).Scan(&mime)
		if e != nil || !strings.HasPrefix(mime, "image/") {
			return x, ErrDenied
		}
	}

	if x.Body.References == nil {
		x.Body.References = []Reference{}
	}
	seen := map[string]bool{}
	for i, r := range x.Body.References {
		key := r.Kind + ":" + r.ID
		if seen[key] {
			return x, fmt.Errorf("%w: duplicate reference", ErrInvalid)
		}
		seen[key] = true
		v, e := s.Resolve(ctx, r.Kind, r.ID)
		if e != nil {
			return x, ErrDenied
		}
		if old != nil {
			for _, prev := range old.Body.References {
				if prev.Kind == r.Kind && prev.ID == r.ID {
					v.Title = prev.Title
					v.Artist = prev.Artist
					break
				}
			}
		}
		x.Body.References[i] = v
	}
	if r := x.Body.Cover; r != nil {
		v, e := s.Resolve(ctx, r.Kind, r.ID)
		if e != nil {
			return x, ErrDenied
		}
		x.Body.Cover = &v
	}
	recipients := map[string]bool{}
	for _, v := range x.SharedWith {
		if v == u.ID || recipients[v] || s.UserExists == nil || !s.UserExists(ctx, v) {
			return x, ErrInvalid
		}
		recipients[v] = true
	}
	body, e := json.Marshal(x.Body)
	if e != nil || len(body) > 65536 {
		return x, ErrInvalid
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return x, e
	}
	defer tx.Rollback()
	var personal, total, previous int64
	if e = tx.QueryRowContext(ctx, `SELECT coalesce(sum(CASE WHEN owner_id=? THEN length(CAST(body AS BLOB)) ELSE 0 END),0),coalesce(sum(length(CAST(body AS BLOB))),0),coalesce(sum(CASE WHEN id=? THEN length(CAST(body AS BLOB)) ELSE 0 END),0) FROM family_entry`, u.ID, x.ID).Scan(&personal, &total, &previous); e != nil {
		return x, e
	}
	if personal-previous+int64(len(body)) > 16*1024*1024 || total-previous+int64(len(body)) > 64*1024*1024 {
		return x, ErrQuota
	}
	if x.ID == "" {
		var n int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_entry WHERE owner_id=?`, u.ID).Scan(&n); e != nil {
			return x, e
		}
		var total int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_entry`).Scan(&total); e != nil {
			return x, e
		}
		if n >= 2000 || total >= 20000 {
			return x, fmt.Errorf("%w: entry quota reached", ErrInvalid)
		}
		x.ID = id.NewRandom()
		_, e = tx.ExecContext(ctx, `INSERT INTO family_entry(id,kind,owner_id,title,body,archived,revision,created_at,updated_at) VALUES(?,?,?,?,?,?,1,?,?)`, x.ID, x.Kind, u.ID, x.Title, string(body), x.Archived, now, now)
	} else {
		res, err := tx.ExecContext(ctx, `UPDATE family_entry SET title=?,body=?,archived=?,revision=revision+1,updated_at=? WHERE id=? AND kind=? AND owner_id=? AND revision=?`, x.Title, string(body), x.Archived, now, x.ID, x.Kind, u.ID, x.Revision)
		e = err
		if e == nil {
			n, _ := res.RowsAffected()
			if n != 1 {
				return x, ErrConflict
			}
		}
	}
	if e != nil {
		return x, e
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM family_access WHERE entry_id=?`, x.ID); e != nil {
		return x, e
	}
	for v := range recipients {
		if _, e = tx.ExecContext(ctx, `INSERT INTO family_access(entry_id,user_id) VALUES(?,?)`, x.ID, v); e != nil {
			return x, e
		}
	}
	if e = tx.Commit(); e != nil {
		return x, e
	}
	return s.Get(ctx, x.ID)
}
