package nativeapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/navidrome/navidrome/conf"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/core/family"
	"github.com/navidrome/navidrome/db"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
)

func familyJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	json.NewEncoder(w).Encode(v)
}
func familyError(w http.ResponseWriter, e error) {
	status := 503
	msg := "Family content temporarily unavailable"
	switch {
	case errors.Is(e, family.ErrQuota):
		status = 413
		msg = "Family attachment or history quota reached"
	case errors.Is(e, family.ErrInvalid):
		status = 400
		msg = "Invalid family content"
	case errors.Is(e, family.ErrDenied), errors.Is(e, model.ErrNotFound):
		status = 404
		msg = "Content unavailable"
	case errors.Is(e, family.ErrConflict):
		status = 409
		msg = e.Error()
	}
	http.Error(w, msg, status)
}
func familyDecode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		http.Error(w, "Invalid request", 400)
		return false
	}
	return true
}
func (api *Router) familyAvailable(ctx context.Context, path string, missing bool) bool {
	if missing {
		return false
	}
	scheme, id, remote := strings.Cut(path, "://")
	if !remote || (scheme != "s3" && scheme != "sftp" && scheme != "webdav") {
		return true
	}
	id = strings.SplitN(id, "/", 2)[0]
	props := api.ds.Property(ctx)
	disabled, e := props.DefaultGet("external.disabled."+id, "false")
	if e != nil || disabled == "true" {
		return false
	}
	status, e := props.DefaultGet("external.status."+id, "")
	return e == nil && status != "Offline"
}
func (api *Router) familyResolve(ctx context.Context, kind, id string) (family.Reference, error) {
	ref := family.Reference{Kind: kind, ID: id}
	switch kind {
	case "track":
		v, e := api.ds.MediaFile(ctx).Get(id)
		if e != nil {
			return ref, e
		}
		ref.Title = v.Title
		ref.Artist = v.Artist
		ref.Available = api.familyAvailable(ctx, v.LibraryPath, v.Missing)
	case "album":
		v, e := api.ds.Album(ctx).Get(id)
		if e != nil {
			return ref, e
		}
		ref.Title = v.Name
		ref.Artist = v.AlbumArtist
		ref.Available = api.familyAvailable(ctx, v.LibraryPath, v.Missing)
	case "playlist":
		v, e := api.ds.Playlist(ctx).Get(id)
		if e != nil {
			return ref, e
		}
		u, _ := request.UserFrom(ctx)
		if v.OwnerID != u.ID && !v.Public {
			return ref, family.ErrDenied
		}
		ref.Title = v.Name
		ref.Available = true
	default:
		return ref, family.ErrInvalid
	}
	return ref, nil
}
func (api *Router) familyStore() family.Store {
	root := ""
	if conf.Server.DataFolder.String() != "" {
		root = filepath.Join(conf.Server.DataFolder.String(), "family-media")
	}
	return family.Store{MediaRoot: root, DB: db.Db(), Resolve: api.familyResolve, IsAdministrator: func(ctx context.Context, id string) bool {
		u, e := api.ds.User(ctx).Get(id)
		return e == nil && u.IsAdmin
	}, UserExists: func(ctx context.Context, id string) bool {
		return api.familyRecipientAllowed(ctx, id)
	}}
}
func (api *Router) addFamilyHomeRoutes(r chi.Router) {
	r.Route("/family", func(r chi.Router) {
		r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
			u, _ := request.UserFrom(r.Context())
			var child *model.FamilyChild
			if u.FamilyChild != nil {
				p := *u.FamilyChild
				p.ParentID = ""
				child = &p
			}
			familyJSON(w, map[string]any{"id": u.ID, "name": u.UserName, "admin": u.IsAdmin, "child": child})
		})
		api.addFamilyInteractionRoutes(r)
		api.addFamilyChildRoutes(r)
		api.addFamilyLegacyRoutes(r)
		api.addFamilyVersionRoutes(r)
		api.addFamilyDoctorRoutes(r)
		r.Get("/search", api.familySearch)
		r.Get("/people", func(w http.ResponseWriter, r *http.Request) {
			users, e := api.ds.User(r.Context()).GetAll(model.QueryOptions{Max: 100})
			if e != nil {
				familyError(w, e)
				return
			}
			out := []map[string]string{}
			for _, u := range users {
				if !api.familyRecipientAllowed(r.Context(), u.ID) {
					continue
				}
				out = append(out, map[string]string{"id": u.ID, "name": u.UserName})
			}
			familyJSON(w, out)
		})
		r.Get("/shelves", func(w http.ResponseWriter, r *http.Request) {
			n, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			v, e := api.familyStore().List(r.Context(), "shelf", r.URL.Query().Get("archived") == "true", n)
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, v)
		})
		r.Post("/shelves", func(w http.ResponseWriter, r *http.Request) {
			var v family.Entry
			if !familyDecode(w, r, &v) {
				return
			}
			if v.ID != "" {
				familyError(w, family.ErrInvalid)
				return
			}
			v.Kind = "shelf"
			x, e := api.familyStore().Save(r.Context(), v)
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, x)
		})
		r.Get("/shelves/{id}", func(w http.ResponseWriter, r *http.Request) {
			x, e := api.familyStore().GetKind(r.Context(), "shelf", chi.URLParam(r, "id"))
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, x)
		})
		r.Put("/shelves/{id}", func(w http.ResponseWriter, r *http.Request) {
			var v family.Entry
			if !familyDecode(w, r, &v) {
				return
			}
			v.ID = chi.URLParam(r, "id")
			v.Kind = "shelf"
			x, e := api.familyStore().Save(r.Context(), v)
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, x)
		})
		r.Get("/preferences", func(w http.ResponseWriter, r *http.Request) {
			v, e := api.familyStore().Preferences(r.Context())
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, v)
		})
		r.Put("/preferences", func(w http.ResponseWriter, r *http.Request) {
			var v family.Preferences
			if !familyDecode(w, r, &v) {
				return
			}
			if e := api.familyStore().SetPreferences(r.Context(), v); e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, v)
		})
		r.Post("/forgotten/{id}/dismiss", func(w http.ResponseWriter, r *http.Request) {
			if e := api.familyStore().Dismiss(r.Context(), chi.URLParam(r, "id")); e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, map[string]bool{"dismissed": true})
		})
		r.Get("/albums/{section}", api.familyAlbums)
		r.Get("/passport/{id}", api.familyPassport)
	})
}
func (api *Router) familyAlbums(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u, _ := request.UserFrom(ctx)
	section := chi.URLParam(r, "section")
	options := model.QueryOptions{Max: 24, Sort: "recently_added", Order: "desc"}
	p, e := api.familyStore().Preferences(ctx)
	if e != nil {
		familyError(w, e)
		return
	}
	switch section {
	case "new":
		options.Filters = sq.Eq{"album.missing": false}
	case "continue":
		options.Sort = "play_date"
		options.Filters = sq.And{sq.Expr("play_date IS NOT NULL"), sq.Eq{"album.missing": false}}
	case "forgotten":
		if !p.ForgottenEnabled {
			familyJSON(w, []family.Reference{})
			return
		}
		cutoff := time.Now().UTC().AddDate(0, 0, -p.ForgottenDays)
		options.Sort = "name"
		options.Order = "asc"
		options.Filters = sq.And{sq.Or{sq.Eq{"play_date": nil}, sq.Lt{"play_date": cutoff}}, sq.Eq{"album.missing": false}, sq.Expr("NOT EXISTS(SELECT 1 FROM family_dismissal fd WHERE fd.user_id=? AND fd.album_id=album.id)", u.ID)}
		if p.ForgottenMode == "favorites" {
			options.Filters = sq.And{options.Filters, sq.Eq{"starred": true}}
		}
		if p.ForgottenMode == "surprise" {
			options.Sort = "random"
			options.Seed = fmt.Sprintf("%s-%d", u.ID, time.Now().UTC().Unix()/604800)
		}
	default:
		familyError(w, family.ErrInvalid)
		return
	}
	offset, e := strconv.Atoi(r.URL.Query().Get("offset"))
	if r.URL.Query().Get("offset") != "" && (e != nil || offset < 0 || offset > 100000) {
		familyError(w, family.ErrInvalid)
		return
	}
	options.Offset = offset
	albums, e := api.ds.Album(ctx).GetAll(options)
	if e != nil {
		familyError(w, e)
		return
	}
	out := []family.Reference{}
	for _, a := range albums {
		if api.familyAvailable(ctx, a.LibraryPath, a.Missing) {
			out = append(out, family.Reference{Kind: "album", ID: a.ID, Title: a.Name, Artist: a.AlbumArtist, Available: true})
		}
	}
	familyJSON(w, out)
}
func familySourceType(path string) string {
	for prefix, label := range map[string]string{"bunny://": "Bunny Storage", "s3://": "S3", "sftp://": "SFTP", "webdav://": "HTTPS WebDAV"} {
		if strings.HasPrefix(path, prefix) {
			return label
		}
	}
	return "Local files"
}
func (api *Router) familyPassport(w http.ResponseWriter, r *http.Request) {
	m, e := api.ds.MediaFile(r.Context()).Get(chi.URLParam(r, "id"))
	if e != nil {
		familyError(w, e)
		return
	}
	codec := strings.ToLower(m.Codec)
	if codec == "" {
		codec = "Unknown"
	}
	compression := "Unknown"
	switch {
	case codec == "flac" || codec == "alac" || strings.HasPrefix(codec, "pcm_") || codec == "wavpack":
		compression = "Lossless codec (not proof of provenance)"
	case codec == "mp3" || codec == "aac" || codec == "opus" || codec == "vorbis":
		compression = "Lossy codec"
	}
	sourceType := familySourceType(m.LibraryPath)
	value := map[string]any{"sourceType": sourceType, "id": m.ID, "title": m.Title, "artist": m.Artist, "libraryId": m.LibraryID, "library": fmt.Sprintf("Library %d", m.LibraryID), "container": m.Suffix, "codec": codec, "compression": compression, "sampleRate": m.SampleRate, "bitDepth": m.BitDepth, "bitRate": m.BitRate, "channels": m.Channels, "duration": m.Duration, "size": m.Size, "available": api.familyAvailable(r.Context(), m.LibraryPath, m.Missing), "metadataUpdatedAt": m.UpdatedAt, "integrity": "Not analyzed", "mastering": "Unknown", "provenance": "Unknown"}
	doctor := api.familyDoctor()
	doctor.Track = func(ctx context.Context, key string) (family.DoctorTrack, error) {
		if key != m.ID {
			return family.DoctorTrack{}, family.ErrDenied
		}
		return api.doctorTrackDTO(ctx, *m), nil
	}
	if analysis, err := doctor.PassportAnalysis(r.Context(), m.ID); err == nil {
		value["analysis"] = analysis
		if analysis["hasSuccessfulEvidence"] == false {
			value["integrity"] = "Investigation pending or incomplete; no successful verification recorded"
		} else if analysis["evidenceStaleOrIncomplete"] == true {
			value["integrity"] = "Older evidence retained; newer verification stale or incomplete"
		} else if analysis["decode"] == "decoded" {
			value["integrity"] = "First audio stream decoded at the recorded verification date"
		} else {
			value["integrity"] = "Byte hash available; full audio decode not verified"
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		value["integrity"] = "Analysis temporarily unavailable"
	}
	familyJSON(w, value)
}

func (api *Router) familySearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	kind := r.URL.Query().Get("kind")
	if len(q) < 2 || len(q) > 100 {
		familyError(w, family.ErrInvalid)
		return
	}
	refs := []family.Reference{}
	opt := model.QueryOptions{Max: 12, Sort: "name"}
	var e error
	switch kind {
	case "track":
		opt.Sort = "title"
		opt.Filters = sq.Like{"media_file.title": "%" + q + "%"}
		var xs model.MediaFiles
		xs, e = api.ds.MediaFile(ctx).GetAll(opt)
		for _, x := range xs {
			v, err := api.familyResolve(ctx, kind, x.ID)
			if err == nil {
				refs = append(refs, v)
			}
		}
	case "album":
		opt.Filters = sq.Like{"album.name": "%" + q + "%"}
		var xs model.Albums
		xs, e = api.ds.Album(ctx).GetAll(opt)
		for _, x := range xs {
			v, err := api.familyResolve(ctx, kind, x.ID)
			if err == nil {
				refs = append(refs, v)
			}
		}
	case "playlist":
		opt.Filters = sq.Like{"playlist.name": "%" + q + "%"}
		var xs model.Playlists
		xs, e = api.ds.Playlist(ctx).GetAll(opt)
		for _, x := range xs {
			v, err := api.familyResolve(ctx, kind, x.ID)
			if err == nil {
				refs = append(refs, v)
			}
		}
	default:
		e = family.ErrInvalid
	}
	if e != nil {
		familyError(w, e)
		return
	}
	familyJSON(w, refs)
}
