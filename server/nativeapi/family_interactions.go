package nativeapi

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/core/family"
	"github.com/navidrome/navidrome/model"
)

func (api *Router) addFamilyInteractionRoutes(r chi.Router) {
	for plural, kind := range map[string]string{"capsules": "capsule", "letters": "letter", "requests": "request"} {
		r.Route("/"+plural, func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				offset, e := strconv.Atoi(r.URL.Query().Get("offset"))
				if e != nil && r.URL.Query().Get("offset") != "" {
					familyError(w, family.ErrInvalid)
					return
				}
				opt := family.ListOptions{Offset: offset, Archived: r.URL.Query().Get("archived") == "true", View: r.URL.Query().Get("view"), Query: r.URL.Query().Get("q"), From: r.URL.Query().Get("from"), To: r.URL.Query().Get("to")}
				xs, e := api.familyStore().ListEntries(r.Context(), kind, opt)
				if e != nil {
					familyError(w, e)
					return
				}
				familyJSON(w, xs)
			})
			r.Post("/", func(w http.ResponseWriter, r *http.Request) {
				var x family.Entry
				if !familyDecode(w, r, &x) {
					return
				}
				if x.ID != "" {
					familyError(w, family.ErrInvalid)
					return
				}
				x.Kind = kind
				if kind == "request" {
					x.SharedWith = nil
				}
				v, e := api.familyStore().Save(r.Context(), x)
				if e != nil {
					familyError(w, e)
					return
				}
				familyJSON(w, v)
			})
			r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
				v, e := api.familyStore().GetKind(r.Context(), kind, chi.URLParam(r, "id"))
				if e != nil {
					familyError(w, e)
					return
				}
				familyJSON(w, v)
			})
			r.Put("/{id}", func(w http.ResponseWriter, r *http.Request) {
				var x family.Entry
				if !familyDecode(w, r, &x) {
					return
				}
				x.ID = chi.URLParam(r, "id")
				x.Kind = kind
				if kind == "request" {
					x.SharedWith = nil
				}
				v, e := api.familyStore().Save(r.Context(), x)
				if e != nil {
					familyError(w, e)
					return
				}
				familyJSON(w, v)
			})
			if kind == "letter" || kind == "request" {
				r.Post("/{id}/archive", func(w http.ResponseWriter, r *http.Request) {
					var v struct {
						Archived bool `json:"archived"`
					}
					if !familyDecode(w, r, &v) {
						return
					}
					x, e := api.familyStore().GetKind(r.Context(), kind, chi.URLParam(r, "id"))
					if e != nil {
						familyError(w, e)
						return
					}
					if e = api.familyStore().ArchiveInbox(r.Context(), x.ID, v.Archived); e != nil {
						familyError(w, e)
						return
					}
					familyJSON(w, map[string]bool{"archived": v.Archived})
				})
			}
			if kind == "letter" || kind == "capsule" {
				r.Get("/{id}/attachments", func(w http.ResponseWriter, r *http.Request) {
					key := chi.URLParam(r, "id")
					if _, e := api.familyStore().GetKind(r.Context(), kind, key); e != nil {
						familyError(w, e)
						return
					}
					v, e := api.familyStore().Attachments(r.Context(), key, r.URL.Query().Get("hidden") == "true")
					if e != nil {
						familyError(w, e)
						return
					}
					familyJSON(w, v)
				})
				r.Post("/{id}/attachments", func(w http.ResponseWriter, r *http.Request) {
					key := chi.URLParam(r, "id")
					if _, e := api.familyStore().GetKind(r.Context(), kind, key); e != nil {
						familyError(w, e)
						return
					}
					v, e := api.familyStore().AddAttachment(r.Context(), key, http.MaxBytesReader(w, r.Body, family.MaxAttachmentBytes+1))
					if e != nil {
						familyError(w, e)
						return
					}
					familyJSON(w, v)
				})
			}
		})
	}
	r.Post("/letters/{id}/send", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Revision int    `json:"revision"`
			Key      string `json:"key"`
			Resend   bool   `json:"resend"`
		}
		if !familyDecode(w, r, &v) {
			return
		}
		x, e := api.familyStore().SendLetter(r.Context(), chi.URLParam(r, "id"), v.Revision, v.Key, v.Resend)
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, x)
	})
	r.Post("/letters/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Revision int `json:"revision"`
		}
		if !familyDecode(w, r, &v) {
			return
		}
		x, e := api.familyStore().CancelLetter(r.Context(), chi.URLParam(r, "id"), v.Revision)
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, x)
	})
	r.Post("/letters/{id}/read", func(w http.ResponseWriter, r *http.Request) {
		if e := api.familyStore().MarkLetterRead(r.Context(), chi.URLParam(r, "id")); e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, map[string]bool{"read": true})
	})
	r.Post("/letters/preview-date", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Day      string `json:"day"`
			Time     string `json:"time"`
			Timezone string `json:"timezone"`
		}
		if !familyDecode(w, r, &v) {
			return
		}
		next, e := family.NextAnniversary(time.Now(), v.Day, v.Time, v.Timezone)
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, map[string]string{"next": next.Format("2006-01-02T15:04:05Z07:00")})
	})
	r.Post("/requests/{id}/submit", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Revision int `json:"revision"`
		}
		if !familyDecode(w, r, &v) {
			return
		}
		users, e := api.ds.User(r.Context()).GetAll(model.QueryOptions{Max: 100})
		if e != nil {
			familyError(w, e)
			return
		}
		owners := []string{}
		for _, u := range users {
			if u.IsAdmin {
				owners = append(owners, u.ID)
			}
		}
		x, e := api.familyStore().SubmitRequest(r.Context(), chi.URLParam(r, "id"), v.Revision, owners)
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, x)
	})
	r.Post("/requests/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Revision   int               `json:"revision"`
			ItemID     string            `json:"itemId"`
			State      string            `json:"state"`
			Comment    string            `json:"comment"`
			Resolution *family.Reference `json:"resolution"`
		}
		if !familyDecode(w, r, &v) {
			return
		}
		x, e := api.familyStore().UpdateRequest(r.Context(), chi.URLParam(r, "id"), v.ItemID, v.State, v.Comment, v.Revision, v.Resolution)
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, x)
	})
	r.Get("/requests/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		v, e := api.familyStore().RequestHistory(r.Context(), chi.URLParam(r, "id"), familyOffset(r))
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, v)
	})
	r.Post("/request-preview", api.familyRequestPreview)
	r.Get("/notifications", func(w http.ResponseWriter, r *http.Request) {
		v, e := api.familyStore().Notifications(r.Context())
		if e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, v)
	})
	r.Post("/notifications/{id}/read", func(w http.ResponseWriter, r *http.Request) {
		if e := api.familyStore().ReadNotification(r.Context(), chi.URLParam(r, "id")); e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, map[string]bool{"read": true})
	})
	r.Get("/attachments/{id}", api.familyAttachment)
	r.Head("/attachments/{id}", api.familyAttachment)
	r.Post("/attachments/{id}/visibility", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Hidden bool `json:"hidden"`
		}
		if !familyDecode(w, r, &v) {
			return
		}
		if e := api.familyStore().HideAttachment(r.Context(), chi.URLParam(r, "id"), v.Hidden); e != nil {
			familyError(w, e)
			return
		}
		familyJSON(w, map[string]bool{"hidden": v.Hidden})
	})
}
func (api *Router) familyAttachment(w http.ResponseWriter, r *http.Request) {
	var a family.Attachment
	var path string
	var e error
	if r.URL.Query().Get("thumb") == "true" {
		a, path, e = api.familyStore().Thumbnail(r.Context(), chi.URLParam(r, "id"))
	} else {
		a, path, e = api.familyStore().Attachment(r.Context(), chi.URLParam(r, "id"))
	}
	if e != nil {
		familyError(w, e)
		return
	}
	file, e := os.Open(path)
	if e != nil {
		familyError(w, e)
		return
	}
	defer file.Close()
	info, e := file.Stat()
	if e != nil || info.Size() != a.Size {
		familyError(w, family.ErrInvalid)
		return
	}
	w.Header().Set("Content-Type", a.MIME)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", `inline; filename="`+a.ID+`"`)
	w.Header().Set("ETag", `"`+a.SHA256+`"`)
	http.ServeContent(w, r, a.ID, info.ModTime(), file)
}
func (api *Router) familyRequestPreview(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Kind  string `json:"kind"`
		Lines string `json:"lines"`
	}
	if !familyDecode(w, r, &v) {
		return
	}
	if v.Kind != "track" && v.Kind != "album" && v.Kind != "artist" || len(v.Lines) > 32000 {
		familyError(w, family.ErrInvalid)
		return
	}
	items := []family.RequestItem{}
	seen := map[string]bool{}
	duplicates := 0
	for _, line := range strings.Split(strings.ReplaceAll(v.Lines, "\r\n", "\n"), "\n") {
		q := strings.TrimSpace(line)
		if q == "" {
			continue
		}
		if len(q) > 300 {
			familyError(w, family.ErrInvalid)
			return
		}
		key := strings.ToLower(strings.Join(strings.Fields(q), " "))
		if seen[key] {
			duplicates++
			continue
		}
		seen[key] = true
		item := family.RequestItem{Kind: v.Kind, Query: q, State: "requested"}
		options := model.QueryOptions{Max: 3}
		switch v.Kind {
		case "track":
			options.Filters = sq.Eq{"media_file.title": q}
			tracks, e := api.ds.MediaFile(r.Context()).GetAll(options)
			if e != nil {
				familyError(w, e)
				return
			}
			for _, track := range tracks {
				ref, e := api.familyResolve(r.Context(), "track", track.ID)
				if e == nil {
					item.Candidates = append(item.Candidates, ref)
				}
			}
		case "album":
			options.Filters = sq.Eq{"album.name": q}
			albums, e := api.ds.Album(r.Context()).GetAll(options)
			if e != nil {
				familyError(w, e)
				return
			}
			for _, album := range albums {
				ref, e := api.familyResolve(r.Context(), "album", album.ID)
				if e == nil {
					item.Candidates = append(item.Candidates, ref)
				}
			}
		}
		items = append(items, item)
		if len(items) > 100 {
			familyError(w, family.ErrInvalid)
			return
		}
	}
	if len(items) == 0 {
		familyError(w, family.ErrInvalid)
		return
	}
	familyJSON(w, map[string]any{"items": items, "duplicates": duplicates, "note": "References only; no downloads or purchases are performed"})
}

func familyOffset(r *http.Request) int {
	v, e := strconv.Atoi(r.URL.Query().Get("offset"))
	if e != nil && r.URL.Query().Get("offset") != "" {
		return -1
	}
	return v
}
