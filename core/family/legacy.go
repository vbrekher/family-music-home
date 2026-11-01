package family

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/navidrome/navidrome/model/id"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const LegacyMaxBytes int64 = 160 * 1024 * 1024
const LegacyMaxArchive int64 = 192 * 1024 * 1024
const LegacyMaxFiles = 10000
const LegacyMaxEntries = 2000
const LegacyMaxMetadata = 32 * 1024 * 1024

type LegacyOptions struct {
	Catalog     bool   `json:"catalog"`
	Playlists   bool   `json:"playlists"`
	Scope       string `json:"scope"`
	From        string `json:"from"`
	To          string `json:"to"`
	Attachments bool   `json:"attachments"`
	Archived    bool   `json:"archived"`
}
type LegacyJob struct {
	ID        string        `json:"id"`
	Options   LegacyOptions `json:"options"`
	EntryIDs  []string      `json:"entryIds"`
	CreatedAt string        `json:"createdAt"`
	ExpiresAt string        `json:"expiresAt"`
}
type LegacyMedia struct {
	Attachment
	File string `json:"file"`
}
type LegacyPlaylist struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Comment    string      `json:"comment"`
	References []Reference `json:"references"`
}
type LegacyManifest struct {
	Playlists     []LegacyPlaylist `json:"playlists"`
	SchemaVersion int              `json:"schemaVersion"`
	ExportedAt    string           `json:"exportedAt"`
	AccountID     string           `json:"accountId"`
	Options       LegacyOptions    `json:"options"`
	Entries       []Entry          `json:"entries"`
	Media         []LegacyMedia    `json:"attachments"`
	Preferences   any              `json:"preferences"`
	Catalog       any              `json:"listeningAndFavoriteSummaries"`
	Versions      any              `json:"editions"`
	AudioIncluded bool             `json:"audioIncluded"`
	Note          string           `json:"note"`
}
type Legacy struct {
	Store     Store
	Now       func() time.Time
	Catalog   func(context.Context) (any, error)
	Playlists func(context.Context, LegacyOptions) ([]LegacyPlaylist, error)
	Versions  func(context.Context, LegacyOptions) (any, error)
}

func (l Legacy) now() time.Time {
	if l.Now != nil {
		return l.Now().UTC()
	}
	return time.Now().UTC()
}
func validLegacyOptions(o LegacyOptions) error {
	if o.Scope != "personal" && o.Scope != "allowed-shared" {
		return ErrInvalid
	}
	for _, s := range []string{o.From, o.To} {
		if s != "" {
			t, e := time.Parse("2006-01-02", s)
			if e != nil || t.Year() < 1 {
				return ErrInvalid
			}
		}
	}
	if o.From != "" && o.To != "" && o.From > o.To {
		return ErrInvalid
	}
	return nil
}
func (l Legacy) Prepare(ctx context.Context, opt LegacyOptions) (LegacyJob, error) {
	u, e := actor(ctx)
	if e != nil {
		return LegacyJob{}, e
	}
	if u.FamilyChild != nil && !u.FamilyChild.Enabled {
		return LegacyJob{}, ErrDenied
	}
	if e = validLegacyOptions(opt); e != nil {
		return LegacyJob{}, e
	}
	query := `SELECT e.id FROM family_entry e WHERE ` + readAccess
	args := []any{u.ID, u.ID, u.ID}
	if opt.Scope == "personal" {
		query += ` AND e.owner_id=?`
		args = append(args, u.ID)
	}
	if !opt.Archived {
		query += ` AND (CASE WHEN e.kind IN ('letter','request') THEN coalesce((SELECT fi.archived FROM family_inbox_state fi WHERE fi.entry_id=e.id AND fi.user_id=?),0) ELSE e.archived END)=0`
		args = append(args, u.ID)
	}
	if opt.From != "" {
		query += ` AND julianday(e.created_at)>=julianday(?)`
		args = append(args, opt.From)
	}
	if opt.To != "" {
		query += ` AND julianday(e.created_at)<julianday(?,'+1 day')`
		args = append(args, opt.To)
	}
	if u.FamilyChild != nil && !u.FamilyChild.AllowFamilyMessages {
		query += ` AND e.owner_id IN (?,?)`
		args = append(args, u.ID, u.FamilyChild.ParentID)
	}
	query += ` ORDER BY e.created_at,e.id LIMIT 2001`
	rows, e := l.Store.DB.QueryContext(ctx, query, args...)
	if e != nil {
		return LegacyJob{}, e
	}
	keys := []string{}
	for rows.Next() {
		var k string
		if e = rows.Scan(&k); e != nil {
			rows.Close()
			return LegacyJob{}, e
		}
		keys = append(keys, k)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return LegacyJob{}, e
	}
	if len(keys) > LegacyMaxEntries {
		return LegacyJob{}, ErrQuota
	}
	now := l.now()
	j := LegacyJob{ID: id.NewRandom(), Options: opt, EntryIDs: keys, CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339Nano)}
	raw, _ := json.Marshal(j)
	if len(raw) > 65536 {
		return j, ErrQuota
	}
	_, e = l.Store.DB.ExecContext(ctx, `INSERT INTO family_legacy_export(id,user_id,options,created_at,expires_at) VALUES(?,?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET id=excluded.id,options=excluded.options,created_at=excluded.created_at,expires_at=excluded.expires_at`, j.ID, u.ID, string(raw), j.CreatedAt, j.ExpiresAt)
	return j, e
}
func (l Legacy) Job(ctx context.Context, key string) (LegacyJob, error) {
	u, e := actor(ctx)
	if e != nil {
		return LegacyJob{}, e
	}
	var raw string
	e = l.Store.DB.QueryRowContext(ctx, `SELECT options FROM family_legacy_export WHERE id=? AND user_id=?`, key, u.ID).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return LegacyJob{}, ErrDenied
	}
	if e != nil {
		return LegacyJob{}, e
	}
	var j LegacyJob
	if e = json.Unmarshal([]byte(raw), &j); e != nil {
		return j, e
	}
	until, e := time.Parse(time.RFC3339Nano, j.ExpiresAt)
	if e != nil || !l.now().Before(until) || j.ID != key {
		return j, ErrDenied
	}
	return j, nil
}
func (l Legacy) Manifest(ctx context.Context, key string) (LegacyManifest, error) {
	u, e := actor(ctx)
	if e != nil {
		return LegacyManifest{}, e
	}
	j, e := l.Job(ctx, key)
	if e != nil {
		return LegacyManifest{}, e
	}
	m := LegacyManifest{SchemaVersion: 1, ExportedAt: l.now().Format(time.RFC3339Nano), AccountID: u.ID, Options: j.Options, Entries: []Entry{}, Media: []LegacyMedia{}, Playlists: []LegacyPlaylist{}, Note: "Private metadata and permitted attachments; original music and system credentials are excluded. Relative music paths are relocation targets, not included audio. Integrity checks do not establish authorship or encryption. Complete server restoration uses the separately documented encrypted database/media backup."}
	for _, key := range j.EntryIDs {
		x, e := l.Store.Get(ctx, key)
		if errors.Is(e, ErrDenied) {
			continue
		}
		if e != nil {
			return m, e
		}
		if j.Options.Scope == "personal" && x.OwnerID != u.ID {
			continue
		}
		m.Entries = append(m.Entries, x)
		if j.Options.Attachments && (x.Kind == "capsule" || x.Kind == "letter") {
			media, e := l.Store.Attachments(ctx, x.ID, true)
			if e != nil {
				return m, e
			}
			for _, a := range media {
				suffix := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "audio/mpeg": ".mp3", "audio/wav": ".wav", "audio/mp4": ".m4a"}[a.MIME]
				if suffix == "" || !opaqueID.MatchString(a.ID) {
					return m, ErrInvalid
				}
				m.Media = append(m.Media, LegacyMedia{Attachment: a, File: "attachments/" + a.ID + suffix})
			}
		}
	}
	m.Preferences, e = l.Store.Preferences(ctx)
	if e != nil {
		return m, e
	}
	if j.Options.Catalog && l.Catalog != nil {
		m.Catalog, e = l.Catalog(ctx)
		if e != nil {
			return m, e
		}
	}
	if l.Versions != nil {
		m.Versions, e = l.Versions(ctx, j.Options)
		if e != nil {
			return m, e
		}
	}
	if j.Options.Playlists && l.Playlists != nil {
		m.Playlists, e = l.Playlists(ctx, j.Options)
		if e != nil {
			return m, e
		}
	}
	raw, e := json.Marshal(m)
	if e != nil {
		return m, e
	}
	if len(raw) > LegacyMaxMetadata || len(m.Media)+len(m.Entries)*2+len(m.Playlists)+3 > LegacyMaxFiles {
		return m, ErrQuota
	}
	var total int64 = int64(len(raw)) + 4096 + int64(len(m.Media)+len(m.Entries)*2+len(m.Playlists)+3)*250
	for _, a := range m.Media {
		total += a.Size + 512
	}
	for _, x := range m.Entries {
		total += int64(1024 + 6*(len(x.Title)+len(x.Body.Description)+len(x.Body.Message)))
		for _, r := range x.Body.References {
			total += int64(512 + 6*(len(r.Title)+len(r.Artist)))
		}
	}
	for _, p := range m.Playlists {
		total += int64(1024 + len(p.Comment) + 6*len(p.Name))
		for _, r := range p.References {
			total += int64(512 + len(r.Title) + len(r.Artist))
		}
	}
	if total > LegacyMaxBytes {
		return m, ErrQuota
	}
	return m, nil
}

var legacyTags = regexp.MustCompile(`<[^>]*>`)

func legacyMessage(s string) string {
	s = strings.NewReplacer("<strong>", "**", "</strong>", "**", "<b>", "**", "</b>", "**", "<em>", "*", "</em>", "*", "<i>", "*", "</i>", "*", "<li>", "- ", "</li>", "\n", "</p>", "\n\n", "<br>", "\n", "<br/>", "\n").Replace(s)
	return markdownText(html.UnescapeString(legacyTags.ReplaceAllString(s, "")))
}
func markdownText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "[", "\\[", "]", "\\]", "`", "\\`", "\x00", "").Replace(s)
}
func (l Legacy) Write(ctx context.Context, m LegacyManifest, out io.Writer) error {
	// A single streaming ZIP, no staged archive or original music in /data.
	z := zip.NewWriter(out)
	hashes := map[string]string{}
	var total int64
	add := func(name string, source io.Reader, limit int64, expected string) error {
		if !safeLegacyPath(name) || len(hashes) >= LegacyMaxFiles-1 {
			return ErrInvalid
		}
		if _, exists := hashes[name]; exists {
			return ErrInvalid
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0600)
		w, e := z.CreateHeader(header)
		if e != nil {
			return e
		}
		hash := sha256.New()
		n, e := io.Copy(io.MultiWriter(w, hash), io.LimitReader(source, limit+1))
		if e != nil {
			return e
		}
		total += n
		if n > limit || total > LegacyMaxBytes {
			return ErrQuota
		}
		digest := hex.EncodeToString(hash.Sum(nil))
		if expected != "" && digest != expected {
			return fmt.Errorf("private attachment integrity changed")
		}
		hashes[name] = digest
		return nil
	}
	raw, e := json.Marshal(m)
	if e != nil {
		return e
	}
	if int64(len(raw)) > LegacyMaxMetadata {
		return ErrQuota
	}
	if e = add("manifest.json", strings.NewReader(string(raw)), LegacyMaxMetadata, ""); e != nil {
		return e
	}
	readme := "# Family music history\n\nSchema 1. See manifest.json for your selected, permission-checked records.\n\nThis ZIP is private and unencrypted. Keep it in encrypted storage. Original audio is not included. M3U entries use relative `music/<track-id>` relocation targets; map your own audio using stable catalog IDs in the manifest. Album/playlist references that are not expanded remain comments and manifest references.\n\nVerify every file with checksums.sha256 or the read-only archive verifier. SHA-256 establishes byte integrity, not authorship. No in-place import is claimed. Full account/database restoration uses the documented encrypted server backup including private family media.\n"
	if e = add("README.md", strings.NewReader(readme), LegacyMaxMetadata, ""); e != nil {
		return e
	}
	for _, x := range m.Entries {
		if e = l.Store.CanRead(ctx, x.ID); e != nil {
			return e
		}
		if !opaqueID.MatchString(x.ID) {
			return ErrInvalid
		}
		text := "# " + markdownText(x.Title) + "\n\n" + markdownText(x.Kind) + " · " + markdownText(x.CreatedAt) + "\n\n" + markdownText(x.Body.Description) + "\n\n" + legacyMessage(x.Body.Message) + "\n\n"
		playlist := "#EXTM3U\n# Original audio is not included; resolve music/<id> using manifest references.\n"
		for _, r := range x.Body.References {
			text += "- " + markdownText(r.Title) + " — " + markdownText(r.Artist) + "\n"
			title := strings.NewReplacer("\n", " ", "\r", " ").Replace(r.Title)
			if r.Kind == "track" && opaqueID.MatchString(r.ID) {
				playlist += "#EXTINF:-1," + title + "\n../music/" + r.ID + "\n"
			} else {
				playlist += "# Unexpanded " + r.Kind + " reference; see manifest.json\n"
			}
		}
		for _, a := range m.Media {
			if a.EntryID == x.ID {
				text += "\n[Private attachment](../" + a.File + ")\n"
			}
		}
		if e = add("memories/"+x.ID+".md", strings.NewReader(text), 65536*2, ""); e != nil {
			return e
		}
		if e = add("collections/"+x.ID+".m3u8", strings.NewReader(playlist), 65536, ""); e != nil {
			return e
		}
	}
	for _, a := range m.Media {
		current, path, e := l.Store.Attachment(ctx, a.ID)
		if e != nil {
			return e
		}
		if current.SHA256 != a.SHA256 || current.Size != a.Size {
			return ErrConflict
		}
		info, e := os.Lstat(path)
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() || info.Size() != a.Size || a.Size > MaxAttachmentBytes {
			return ErrInvalid
		}
		f, e := os.Open(filepath.Clean(path))
		if e != nil {
			return e
		}
		opened, statErr := f.Stat()
		if statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) || opened.Size() != a.Size {
			f.Close()
			return ErrInvalid
		}
		e = add(a.File, f, a.Size, a.SHA256)
		f.Close()
		if e != nil {
			return e
		}
	}
	for _, p := range m.Playlists {
		if !opaqueID.MatchString(p.ID) {
			return ErrInvalid
		}
		text := "#EXTM3U\n# Audio is not included; resolve relative targets with manifest.json.\n"
		for _, r := range p.References {
			if r.Kind == "track" && opaqueID.MatchString(r.ID) {
				text += "#EXTINF:-1," + strings.NewReplacer("\n", " ", "\r", " ").Replace(r.Title) + "\n../music/" + r.ID + "\n"
			} else {
				text += "# Unavailable reference; see manifest.json\n"
			}
		}
		if e = add("playlists/"+p.ID+".m3u8", strings.NewReader(text), LegacyMaxMetadata, ""); e != nil {
			return e
		}
	}
	names := make([]string, 0, len(hashes))
	for name := range hashes {
		names = append(names, name)
	}
	sort.Strings(names)
	var sum strings.Builder
	for _, name := range names {
		fmt.Fprintf(&sum, "%s  %s\n", hashes[name], name)
	}
	header := &zip.FileHeader{Name: "checksums.sha256", Method: zip.Deflate}
	header.SetMode(0600)
	w, e := z.CreateHeader(header)
	if e != nil {
		return e
	}
	if _, e = io.WriteString(w, sum.String()); e != nil {
		return e
	}
	return z.Close()
}
