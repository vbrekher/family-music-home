package family

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/navidrome/navidrome/model/id"
)

type VersionTechnical struct {
	Codec      string `json:"codec"`
	Container  string `json:"container"`
	SampleRate int    `json:"sampleRate"`
	BitDepth   *int   `json:"bitDepth"`
	Channels   int    `json:"channels"`
}
type VersionAlbum struct {
	SourceType string `json:"sourceType"`
	Reference
	ReleaseGroup  string             `json:"releaseGroup,omitempty"`
	ReleaseID     string             `json:"releaseId,omitempty"`
	ReleaseDate   string             `json:"releaseDate,omitempty"`
	Edition       string             `json:"edition,omitempty"`
	Label         string             `json:"label,omitempty"`
	ReleaseType   string             `json:"releaseType,omitempty"`
	Library       string             `json:"library"`
	FirstTrackID  string             `json:"firstTrackId,omitempty"`
	Technical     []VersionTechnical `json:"technical"`
	SampledTracks int                `json:"sampledTracks"`
	TotalTracks   int                `json:"totalTracks"`
}
type VersionGroup struct {
	ID           string         `json:"id"`
	Basis        string         `json:"basis"`
	ReleaseGroup string         `json:"releaseGroup,omitempty"`
	Members      []VersionAlbum `json:"members"`
	Preferred    string         `json:"preferred,omitempty"`
	CanEdit      bool           `json:"canEdit"`
	Archived     bool           `json:"archived"`
	Revision     int            `json:"revision"`
}
type Versions struct {
	DB    *sql.DB
	Album func(context.Context, string) (VersionAlbum, error)
}

var mbidPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

func (v Versions) Get(ctx context.Context, key string) (VersionGroup, error) {
	u, e := actor(ctx)
	if e != nil {
		return VersionGroup{}, e
	}
	var x VersionGroup
	var owner string
	e = v.DB.QueryRowContext(ctx, `SELECT id,owner_id,basis,release_group,archived,revision FROM family_version_group WHERE id=?`, key).Scan(&x.ID, &owner, &x.Basis, &x.ReleaseGroup, &x.Archived, &x.Revision)
	if errors.Is(e, sql.ErrNoRows) {
		return x, ErrDenied
	}
	if e != nil {
		return x, e
	}
	x.CanEdit = u.IsAdmin && owner == u.ID
	x.Members = []VersionAlbum{}
	rows, e := v.DB.QueryContext(ctx, `SELECT album_id FROM family_version_member WHERE group_id=? ORDER BY album_id`, key)
	if e != nil {
		return x, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return x, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return x, e
	}
	for _, key := range ids {
		album, e := v.Album(ctx, key)
		if e == nil {
			x.Members = append(x.Members, album)
		}
	}
	if len(x.Members) == 0 {
		return VersionGroup{}, ErrDenied
	}
	var preferred string
	e = v.DB.QueryRowContext(ctx, `SELECT album_id FROM family_version_preference WHERE group_id=? AND user_id=?`, x.ID, u.ID).Scan(&preferred)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return x, e
	}
	for _, album := range x.Members {
		if album.ID == preferred {
			x.Preferred = preferred
		}
	}
	return x, nil
}

// List filters through the same canonical album/library grants before pagination;
// Get resolves every member again, including when a grant changes concurrently.
func (v Versions) List(ctx context.Context, archived bool, offset int) ([]VersionGroup, error) {
	u, e := actor(ctx)
	if e != nil {
		return nil, e
	}
	if offset < 0 || offset > 10000 {
		return nil, ErrInvalid
	}
	query := `SELECT g.id FROM family_version_group g WHERE g.archived=? AND EXISTS(SELECT 1 FROM family_version_member m JOIN album a ON a.id=m.album_id WHERE m.group_id=g.id`
	args := []any{archived}
	if !u.IsAdmin {
		query += ` AND a.library_id IN (SELECT library_id FROM user_library WHERE user_id=?)`
		args = append(args, u.ID)
	}
	query += `) ORDER BY g.updated_at DESC,g.id LIMIT 25 OFFSET ?`
	args = append(args, offset)
	rows, e := v.DB.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var key string
		if e = rows.Scan(&key); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, key)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []VersionGroup{}
	for _, key := range ids {
		x, e := v.Get(ctx, key)
		if errors.Is(e, ErrDenied) {
			continue
		}
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, nil
}
func (v Versions) Create(ctx context.Context, ids []string, basis string, confirmed bool) (VersionGroup, error) {
	u, e := actor(ctx)
	if e != nil {
		return VersionGroup{}, e
	}
	if !u.IsAdmin {
		return VersionGroup{}, ErrDenied
	}
	if len(ids) < 2 || len(ids) > 20 || basis != "musicbrainz" && basis != "confirmed" || basis == "confirmed" && !confirmed {
		return VersionGroup{}, ErrInvalid
	}
	seen := map[string]bool{}
	group := ""
	for _, key := range ids {
		if seen[key] {
			return VersionGroup{}, ErrInvalid
		}
		seen[key] = true
		a, e := v.Album(ctx, key)
		if e != nil {
			return VersionGroup{}, ErrDenied
		}
		if basis == "musicbrainz" {
			if !mbidPattern.MatchString(a.ReleaseGroup) {
				return VersionGroup{}, ErrInvalid
			}
			g := strings.ToLower(a.ReleaseGroup)
			if group != "" && group != g {
				return VersionGroup{}, ErrInvalid
			}
			group = g
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	key := id.NewRandom()
	tx, e := v.DB.BeginTx(ctx, nil)
	if e != nil {
		return VersionGroup{}, e
	}
	defer tx.Rollback()
	existing := false
	if group != "" {
		var owner string
		e = tx.QueryRowContext(ctx, `SELECT id,owner_id FROM family_version_group WHERE basis='musicbrainz' AND release_group=?`, group).Scan(&key, &owner)
		if e == nil {
			if owner != u.ID {
				return VersionGroup{}, ErrDenied
			}
			existing = true
		} else if !errors.Is(e, sql.ErrNoRows) {
			return VersionGroup{}, e
		}
	}
	if !existing {
		var count, total int
		if e = tx.QueryRowContext(ctx, `SELECT count(CASE WHEN owner_id=? THEN 1 END),count(*) FROM family_version_group`, u.ID).Scan(&count, &total); e != nil {
			return VersionGroup{}, e
		}
		if count >= 200 || total >= 2000 {
			return VersionGroup{}, ErrQuota
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO family_version_group(id,owner_id,basis,release_group,created_at,updated_at) VALUES(?,?,?,?,?,?)`, key, u.ID, basis, group, now, now); e != nil {
			return VersionGroup{}, e
		}
	}
	for _, album := range ids {
		if _, e = tx.ExecContext(ctx, `INSERT OR IGNORE INTO family_version_member(group_id,album_id) VALUES(?,?)`, key, album); e != nil {
			return VersionGroup{}, e
		}
	}
	var count int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM family_version_member WHERE group_id=?`, key).Scan(&count); e != nil {
		return VersionGroup{}, e
	}
	if count > 20 {
		return VersionGroup{}, ErrQuota
	}
	if existing {
		if _, e = tx.ExecContext(ctx, `UPDATE family_version_group SET revision=revision+1,updated_at=? WHERE id=?`, now, key); e != nil {
			return VersionGroup{}, e
		}
	}
	if e = tx.Commit(); e != nil {
		return VersionGroup{}, e
	}
	return v.Get(ctx, key)
}
func (v Versions) Prefer(ctx context.Context, key, albumID string) error {
	u, e := actor(ctx)
	if e != nil {
		return e
	}
	x, e := v.Get(ctx, key)
	if e != nil {
		return e
	}
	if albumID == "" {
		_, e = v.DB.ExecContext(ctx, `DELETE FROM family_version_preference WHERE group_id=? AND user_id=?`, key, u.ID)
		return e
	}
	found := false
	for _, a := range x.Members {
		found = found || a.ID == albumID
	}
	if !found {
		return ErrDenied
	}
	_, e = v.DB.ExecContext(ctx, `INSERT INTO family_version_preference(user_id,group_id,album_id) VALUES(?,?,?) ON CONFLICT(user_id,group_id) DO UPDATE SET album_id=excluded.album_id`, u.ID, key, albumID)
	return e
}
func (v Versions) Archive(ctx context.Context, key string, revision int, archived bool) (VersionGroup, error) {
	u, e := actor(ctx)
	if e != nil {
		return VersionGroup{}, e
	}
	if !u.IsAdmin {
		return VersionGroup{}, ErrDenied
	}
	res, e := v.DB.ExecContext(ctx, `UPDATE family_version_group SET archived=?,revision=revision+1,updated_at=? WHERE id=? AND owner_id=? AND revision=?`, archived, time.Now().UTC().Format(time.RFC3339Nano), key, u.ID, revision)
	if e != nil {
		return VersionGroup{}, e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return VersionGroup{}, ErrConflict
	}
	return v.Get(ctx, key)
}
