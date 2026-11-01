package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	sq "github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/core/family"
	"github.com/navidrome/navidrome/db"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/id"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/persistence"
	"github.com/pocketbase/dbx"
	"net/http"
	"time"
	"unicode/utf8"
)

type childWrite struct {
	UserName   string            `json:"userName"`
	Name       string            `json:"name"`
	Password   string            `json:"password"`
	Policy     model.FamilyChild `json:"policy"`
	LibraryIDs []int             `json:"libraryIds"`
	Revision   int               `json:"revision"`
	Confirmed  bool              `json:"confirmed"`
}

func (api *Router) childProfiles(ctx context.Context) ([]family.ChildProfile, error) {
	rows, e := family.ChildProfiles(ctx, db.Db())
	if e != nil {
		return nil, e
	}
	for i := range rows {
		u, e := api.ds.User(ctx).Get(rows[i].UserID)
		if e != nil {
			return nil, e
		}
		rows[i].LibraryIDs = []int{}
		for _, l := range u.Libraries {
			rows[i].LibraryIDs = append(rows[i].LibraryIDs, l.ID)
		}
	}
	return rows, nil
}
func (api *Router) saveChild(ctx context.Context, key string, b childWrite) (family.ChildProfile, error) {
	owner, ok := request.UserFrom(ctx)
	if !ok || !owner.IsAdmin || owner.FamilyChild != nil {
		return family.ChildProfile{}, family.ErrDenied
	}
	if !b.Confirmed || family.ValidateChildPolicy(b.Policy, b.LibraryIDs) != nil || b.Policy.ParentID != "" && b.Policy.ParentID != owner.ID {
		return family.ChildProfile{}, family.ErrInvalid
	}
	if e := family.ValidateChildName(b.UserName, b.Name); e != nil {
		return family.ChildProfile{}, e
	}
	for _, key := range b.LibraryIDs {
		if _, e := api.ds.Library(ctx).Get(key); e != nil {
			return family.ChildProfile{}, family.ErrInvalid
		}
	}
	creating := key == ""
	if creating {
		if utf8.RuneCountInString(b.Password) < 12 || len(b.Password) > 1024 {
			return family.ChildProfile{}, family.ErrInvalid
		}
		key = id.NewRandom()
	} else if b.Password != "" {
		return family.ChildProfile{}, family.ErrInvalid
	}
	b.Policy.ParentID = ""
	raw, _ := json.Marshal(b.Policy)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	e := persistence.WithFamilyTransaction(api.ds, func(tx model.DataStore, sql dbx.Builder) error {
		if creating {
			var count struct{ N int }
			if e := sql.NewQuery(`SELECT count(*) AS n FROM family_child_profile`).One(&count); e != nil {
				return e
			}
			if count.N >= 10 {
				return family.ErrQuota
			}
			_, e := tx.User(ctx).FindByUsername(b.UserName)
			if e == nil {
				return family.ErrConflict
			}
			if !errors.Is(e, model.ErrNotFound) {
				return e
			}
			u := model.User{ID: key, UserName: b.UserName, Name: b.Name, NewPassword: b.Password, IsAdmin: false}
			if e = tx.User(ctx).Put(&u); e != nil {
				return e
			}
			u.NewPassword = ""
			b.Password = ""
			if e = tx.User(ctx).SetUserLibraries(key, b.LibraryIDs); e != nil {
				return e
			}
			_, e = sql.NewQuery(`INSERT INTO family_child_profile(user_id,parent_id,body,created_at,updated_at) VALUES({:id},{:parent},{:body},{:now},{:now})`).Bind(dbx.Params{"id": key, "parent": owner.ID, "body": string(raw), "now": now}).Execute()
			return e
		}
		u, e := tx.User(ctx).Get(key)
		if e != nil || u.FamilyChild == nil || u.FamilyChild.ParentID != owner.ID || u.IsAdmin {
			return family.ErrDenied
		}
		if u.UserName != b.UserName {
			return family.ErrInvalid
		}
		result, e := sql.NewQuery(`UPDATE family_child_profile SET body={:body},revision=revision+1,updated_at={:now} WHERE user_id={:id} AND parent_id={:parent} AND revision={:revision}`).Bind(dbx.Params{"id": key, "parent": owner.ID, "body": string(raw), "now": now, "revision": b.Revision}).Execute()
		if e != nil {
			return e
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return family.ErrConflict
		}
		u.Name = b.Name
		u.NewPassword = ""
		if e = tx.User(ctx).Put(u); e != nil {
			return e
		}
		return tx.User(ctx).SetUserLibraries(key, b.LibraryIDs)
	})
	if e != nil {
		return family.ChildProfile{}, e
	}
	profiles, e := api.childProfiles(ctx)
	for _, p := range profiles {
		if p.UserID == key {
			return p, e
		}
	}
	return family.ChildProfile{}, family.ErrDenied
}
func (api *Router) addFamilyChildRoutes(r chi.Router) {
	r.Get("/children", func(w http.ResponseWriter, r *http.Request) {
		p, e := api.childProfiles(r.Context())
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, p)
	})
	r.Post("/children", func(w http.ResponseWriter, r *http.Request) {
		var b childWrite
		if !familyDecode(w, r, &b) {
			return
		}
		p, e := api.saveChild(r.Context(), "", b)
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, p)
	})
	r.Put("/children/{id}", func(w http.ResponseWriter, r *http.Request) {
		var b childWrite
		if !familyDecode(w, r, &b) {
			return
		}
		p, e := api.saveChild(r.Context(), chi.URLParam(r, "id"), b)
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, p)
	})
	r.Get("/kids", api.familyKids)
}
func (api *Router) familyKids(w http.ResponseWriter, r *http.Request) {
	u, ok := request.UserFrom(r.Context())
	if !ok {
		familyError(w, family.ErrDenied)
		return
	}
	if u.FamilyChild == nil {
		familyJSON(w, map[string]any{"profile": nil, "tracks": []any{}, "shelves": []any{}})
		return
	}
	policy := *u.FamilyChild
	policy.ParentID = ""
	if !policy.Enabled {
		familyJSON(w, map[string]any{"profile": policy, "tracks": []any{}, "shelves": []any{}})
		return
	}
	section := r.URL.Query().Get("section")
	if section == "" {
		section = "songs"
	}
	if section != "songs" && section != "stories" && section != "favorites" && section != "collections" {
		familyError(w, family.ErrInvalid)
		return
	}
	if section == "collections" {
		s, e := api.familyStore().List(r.Context(), "shelf", false, familyOffset(r))
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, map[string]any{"profile": policy, "tracks": []any{}, "shelves": s})
		return
	}
	allowed := []int{}
	story := map[int]bool{}
	for _, id := range policy.StoryLibraries {
		story[id] = true
	}
	for _, lib := range u.Libraries {
		if section == "favorites" || section == "stories" && story[lib.ID] || section == "songs" && !story[lib.ID] {
			allowed = append(allowed, lib.ID)
		}
	}
	out := []map[string]any{}
	if len(allowed) > 0 {
		filters := sq.And{sq.Eq{"media_file.library_id": allowed, "media_file.missing": false}}
		if section == "favorites" {
			filters = append(filters, sq.Eq{"starred": true})
		}
		tracks, e := api.ds.MediaFile(r.Context()).GetAll(model.QueryOptions{Filters: filters, Sort: "title", Order: "asc", Max: 24, Offset: familyOffset(r)})
		if e != nil {
			familyError(w, e)
			return
		}
		for _, m := range tracks {
			out = append(out, map[string]any{"id": m.ID, "title": m.Title, "artist": m.Artist, "album": m.Album, "albumId": m.AlbumID, "duration": m.Duration, "starred": m.Starred, "available": api.familyAvailable(r.Context(), m.LibraryPath, m.Missing)})
		}
	}
	familyJSON(w, map[string]any{"profile": policy, "tracks": out, "shelves": []any{}})
}

// Allowed sharing recipients are narrower than account existence for protected users.
func (api *Router) familyRecipientAllowed(ctx context.Context, key string) bool {
	sender, ok := request.UserFrom(ctx)
	if !ok {
		return false
	}
	recipient, e := api.ds.User(ctx).Get(key)
	if e != nil {
		return false
	}
	if sender.FamilyChild != nil && (!sender.FamilyChild.Enabled || !sender.FamilyChild.AllowFamilyMessages && key != sender.FamilyChild.ParentID) {
		return false
	}
	return recipient.FamilyChild == nil || recipient.FamilyChild.Enabled && (recipient.FamilyChild.AllowFamilyMessages || recipient.FamilyChild.ParentID == sender.ID)
}
