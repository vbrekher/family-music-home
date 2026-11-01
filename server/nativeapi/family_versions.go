package nativeapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/core/family"
	"github.com/navidrome/navidrome/db"
	"github.com/navidrome/navidrome/model"
)

func (api *Router) familyVersionAlbum(ctx context.Context, key string) (family.VersionAlbum, error) {
	a, e := api.ds.Album(ctx).Get(key)
	if e != nil {
		return family.VersionAlbum{}, e
	}
	ref, e := api.familyResolve(ctx, "album", key)
	if e != nil {
		return family.VersionAlbum{}, e
	}
	x := family.VersionAlbum{Reference: ref, ReleaseGroup: a.MbzReleaseGroupID, ReleaseID: a.MbzAlbumID, ReleaseDate: a.ReleaseDate, Edition: strings.Join(a.Tags[model.TagAlbumVersion], "; "), Label: strings.Join(a.Tags[model.TagRecordLabel], "; "), ReleaseType: a.MbzAlbumType, Library: fmt.Sprintf("Library %d", a.LibraryID), SourceType: familySourceType(a.LibraryPath), TotalTracks: a.SongCount, Technical: []family.VersionTechnical{}}
	if x.ReleaseDate == "" {
		x.ReleaseDate = a.Date
	}
	if x.Edition == "" {
		x.Edition = a.MbzAlbumComment
	}
	tracks, e := api.ds.MediaFile(ctx).GetAll(model.QueryOptions{Filters: sq.Eq{"media_file.album_id": key}, Max: 50, Sort: "id"})
	if e != nil {
		return x, e
	}
	x.SampledTracks = len(tracks)
	seen := map[string]bool{}
	for _, m := range tracks {
		if x.FirstTrackID == "" {
			x.FirstTrackID = m.ID
		}
		depth := "unknown"
		if m.BitDepth != nil {
			depth = strconv.Itoa(*m.BitDepth)
		}
		k := strings.Join([]string{m.Codec, m.Suffix, strconv.Itoa(m.SampleRate), depth, strconv.Itoa(m.Channels)}, ":")
		if seen[k] {
			continue
		}
		seen[k] = true
		x.Technical = append(x.Technical, family.VersionTechnical{Codec: m.Codec, Container: m.Suffix, SampleRate: m.SampleRate, BitDepth: m.BitDepth, Channels: m.Channels})
	}
	return x, nil
}
func (api *Router) familyVersions() family.Versions {
	return family.Versions{DB: db.Db(), Album: api.familyVersionAlbum}
}
func (api *Router) addFamilyVersionRoutes(r chi.Router) {
	r.Get("/versions", func(w http.ResponseWriter, r *http.Request) {
		xs, e := api.familyVersions().List(r.Context(), r.URL.Query().Get("archived") == "true", familyOffset(r))
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, xs)
	})
	r.Get("/versions/{id}", func(w http.ResponseWriter, r *http.Request) {
		x, e := api.familyVersions().Get(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, x)
	})
	r.Post("/versions", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AlbumIDs  []string `json:"albumIds"`
			Basis     string   `json:"basis"`
			Confirmed bool     `json:"confirmed"`
		}
		if !familyDecode(w, r, &body) {
			return
		}
		x, e := api.familyVersions().Create(r.Context(), body.AlbumIDs, body.Basis, body.Confirmed)
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, x)
	})
	r.Post("/versions/discover", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AlbumID string `json:"albumId"`
		}
		if !familyDecode(w, r, &body) {
			return
		}
		album, e := api.ds.Album(r.Context()).Get(body.AlbumID)
		if e != nil {
			familyError(w, e)
			return
		}
		if album.MbzReleaseGroupID == "" {
			familyError(w, family.ErrInvalid)
			return
		}
		albums, e := api.ds.Album(r.Context()).GetAll(model.QueryOptions{Filters: sq.Eq{"album.mbz_release_group_id": album.MbzReleaseGroupID}, Max: 21})
		if e != nil {
			familyError(w, e)
			return
		}
		ids := []string{}
		for _, a := range albums {
			ids = append(ids, a.ID)
		}
		x, e := api.familyVersions().Create(r.Context(), ids, "musicbrainz", false)
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, x)
	})
	r.Put("/versions/{id}/preference", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AlbumID string `json:"albumId"`
		}
		if !familyDecode(w, r, &body) {
			return
		}
		if e := api.familyVersions().Prefer(r.Context(), chi.URLParam(r, "id"), body.AlbumID); e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, map[string]bool{"saved": true})
	})
	r.Post("/versions/{id}/archive", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Revision int  `json:"revision"`
			Archived bool `json:"archived"`
		}
		if !familyDecode(w, r, &body) {
			return
		}
		x, e := api.familyVersions().Archive(r.Context(), chi.URLParam(r, "id"), body.Revision, body.Archived)
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, x)
	})
}
