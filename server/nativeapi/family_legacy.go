package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	sq "github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/core/family"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"io"
	"net/http"
	"os"
	"time"
)

var legacySlots = make(chan struct{}, 1)

func (api *Router) familyLegacy() family.Legacy {
	return family.Legacy{Store: api.familyStore(), Catalog: api.legacyCatalog, Playlists: api.legacyPlaylists, Versions: func(ctx context.Context, opt family.LegacyOptions) (any, error) {
		u, _ := request.UserFrom(ctx)
		out := []family.VersionGroup{}
		size := 0
		states := []bool{false}
		if opt.Archived {
			states = append(states, true)
		}
		for _, archived := range states {
			for offset := 0; offset <= 2000; offset += 25 {
				page, e := api.familyVersions().List(ctx, archived, offset)
				if e != nil {
					return nil, e
				}
				for _, g := range page {
					var owner string
					if e := api.familyStore().DB.QueryRowContext(ctx, `SELECT owner_id FROM family_version_group WHERE id=?`, g.ID).Scan(&owner); e != nil {
						return nil, e
					}
					if opt.Scope == "allowed-shared" || owner == u.ID || g.Preferred != "" {
						raw, _ := json.Marshal(g)
						size += len(raw)
						if size > 8*1024*1024 || len(out) >= 2000 {
							return nil, family.ErrQuota
						}
						out = append(out, g)
					}
				}
				if len(page) < 25 {
					break
				}
			}
		}
		return out, nil
	}}
}

func (api *Router) legacyCatalog(ctx context.Context) (any, error) {
	filters := sq.Or{sq.Eq{"starred": true}, sq.Gt{"play_count": 0}, sq.Gt{"rating": 0}}
	out := []map[string]any{}
	size := 0
	add := func(row map[string]any) error {
		raw, e := json.Marshal(row)
		if e != nil {
			return e
		}
		size += len(raw)
		if size > 8*1024*1024 || len(out) >= 10000 {
			return family.ErrQuota
		}
		out = append(out, row)
		return nil
	}
	for offset := 0; offset <= 5000; offset += 50 {
		tracks, e := api.ds.MediaFile(ctx).GetAll(model.QueryOptions{Filters: filters, Sort: "id", Max: 50, Offset: offset})
		if e != nil {
			return nil, e
		}
		if offset == 5000 && len(tracks) > 0 {
			return nil, family.ErrQuota
		}
		for _, m := range tracks {
			if len(m.Title) > 4096 || len(m.Artist) > 4096 {
				return nil, family.ErrQuota
			}
			if e = add(map[string]any{"kind": "track", "id": m.ID, "title": m.Title, "artist": m.Artist, "albumId": m.AlbumID, "annotations": legacyAnnotations(m.Annotations), "codec": m.Codec, "container": m.Suffix, "sampleRate": m.SampleRate, "bitDepth": m.BitDepth, "channels": m.Channels, "duration": m.Duration, "provenance": "Unknown"}); e != nil {
				return nil, e
			}
		}
		if len(tracks) < 50 {
			break
		}
	}
	for offset := 0; offset <= 5000; offset += 50 {
		albums, e := api.ds.Album(ctx).GetAll(model.QueryOptions{Filters: filters, Sort: "id", Max: 50, Offset: offset})
		if e != nil {
			return nil, e
		}
		if offset == 5000 && len(albums) > 0 {
			return nil, family.ErrQuota
		}
		for _, a := range albums {
			if len(a.Name) > 4096 || len(a.AlbumArtist) > 4096 {
				return nil, family.ErrQuota
			}
			if e = add(map[string]any{"kind": "album", "id": a.ID, "title": a.Name, "artist": a.AlbumArtist, "annotations": legacyAnnotations(a.Annotations)}); e != nil {
				return nil, e
			}
		}
		if len(albums) < 50 {
			break
		}
	}
	return out, nil
}
func (api *Router) legacyPlaylists(ctx context.Context, opt family.LegacyOptions) ([]family.LegacyPlaylist, error) {
	u, _ := request.UserFrom(ctx)
	var filters sq.Sqlizer = sq.Eq{"owner_id": u.ID}
	if opt.Scope == "allowed-shared" {
		filters = sq.Or{sq.Eq{"owner_id": u.ID}, sq.Eq{"public": true}}
	}
	list, e := api.ds.Playlist(ctx).GetAll(model.QueryOptions{Filters: filters, Sort: "id", Max: 501})
	if e != nil {
		return nil, e
	}
	if len(list) > 500 {
		return nil, family.ErrQuota
	}
	out := []family.LegacyPlaylist{}
	count, size := 0, 0
	for _, p := range list {
		if p.OwnerID != u.ID && (opt.Scope != "allowed-shared" || !p.Public) {
			continue
		}
		if len(p.Name) > 1024 || len(p.Comment) > 65536 {
			return nil, family.ErrQuota
		}
		row := family.LegacyPlaylist{ID: p.ID, Name: p.Name, Comment: p.Comment, References: []family.Reference{}}
		repo := api.ds.Playlist(ctx).Tracks(p.ID, false)
		if repo == nil {
			return nil, family.ErrDenied
		}
		for offset := 0; offset <= 5000; offset += 50 {
			page, e := repo.GetAll(model.QueryOptions{Max: 50, Offset: offset, Sort: "id", Order: "asc"})
			if e != nil {
				return nil, e
			}
			for _, t := range page {
				count++
				if count > 5000 || len(t.Title) > 4096 || len(t.Artist) > 4096 {
					return nil, family.ErrQuota
				}
				ref, e := api.familyResolve(ctx, "track", t.MediaFileID)
				if e != nil {
					row.References = append(row.References, family.Reference{Kind: "track", Title: "Unavailable item"})
				} else {
					row.References = append(row.References, ref)
				}
			}
			if len(page) < 50 {
				break
			}
		}
		raw, _ := json.Marshal(row)
		size += len(raw)
		if size > 8*1024*1024 {
			return nil, family.ErrQuota
		}
		out = append(out, row)
	}
	return out, nil
}
func (api *Router) addFamilyLegacyRoutes(r chi.Router) {
	r.Post("/legacy/prepare", func(w http.ResponseWriter, r *http.Request) {
		var opt family.LegacyOptions
		if !familyDecode(w, r, &opt) {
			return
		}
		j, e := api.familyLegacy().Prepare(r.Context(), opt)
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, j)
	})
	r.Get("/legacy/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		select {
		case legacySlots <- struct{}{}:
			defer func() { <-legacySlots }()
		default:
			http.Error(w, "Another archive operation is active; try again shortly", 429)
			return
		}
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(2 * time.Minute))
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		l := api.familyLegacy()
		m, e := l.Manifest(ctx, chi.URLParam(r, "id"))
		if e != nil {
			familyError(w, e)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="family-history.zip"`)
		if e = l.Write(ctx, m, w); e != nil {
			panic(http.ErrAbortHandler)
		} // interrupted ZIP is never presented as verified
	})
	r.Post("/legacy/verify", func(w http.ResponseWriter, r *http.Request) {
		select {
		case legacySlots <- struct{}{}:
			defer func() { <-legacySlots }()
		default:
			http.Error(w, "Another archive operation is active; try again shortly", 429)
			return
		}
		_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(2 * time.Minute))
		// One bounded temporary ZIP; members are never extracted to the filesystem.
		f, e := os.CreateTemp("", "family-archive-*.zip")
		if e != nil {
			familyError(w, e)
			return
		}
		defer os.Remove(f.Name())
		defer f.Close()
		n, e := io.Copy(f, http.MaxBytesReader(w, r.Body, family.LegacyMaxArchive))
		if e != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(e, &tooLarge) {
				familyError(w, family.ErrQuota)
			} else {
				familyError(w, family.ErrInvalid)
			}
			return
		}
		result, e := family.VerifyLegacy(f, n)
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, result)
	})
}

func legacyAnnotations(a model.Annotations) any {
	return map[string]any{"starred": a.Starred, "starredAt": a.StarredAt, "rating": a.Rating, "ratedAt": a.RatedAt, "playCount": a.PlayCount, "playDate": a.PlayDate}
}
