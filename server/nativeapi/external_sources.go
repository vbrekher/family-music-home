package nativeapi

import (
	"context"
	"encoding/json"
	"github.com/deluan/rest"
	"github.com/go-chi/chi/v5"
	externaladapter "github.com/navidrome/navidrome/adapters/external"
	"github.com/navidrome/navidrome/core/storage"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"io/fs"
	"net/http"
	"strconv"
	"time"
)

func (api *Router) addExternalSourcesRoute(r chi.Router) {
	externaladapter.BindDataStore(api.ds)
	r.Get("/external-source", func(w http.ResponseWriter, r *http.Request) {
		cfg, e := externaladapter.Sources()
		if e != nil {
			http.Error(w, "Source configuration unavailable", 503)
			return
		}
		props := api.ds.Property(r.Context())
		libs, e := api.ds.Library(r.Context()).GetAll()
		if e != nil {
			http.Error(w, "Catalog unavailable", 503)
			return
		}
		out := []map[string]any{}
		for _, s := range cfg.Sources {
			disabled, _ := props.DefaultGet("external.disabled."+s.ID, "false")
			status, _ := props.DefaultGet("external.status."+s.ID, "Not tested")
			item := map[string]any{"id": s.ID, "type": s.Type, "enabled": s.Enabled && disabled != "true", "status": status, "uri": s.Type + "://" + s.ID, "libraryId": 0}
			for _, lib := range libs {
				if lib.Path == s.Type+"://"+s.ID {
					item["libraryId"] = lib.ID
					item["lastSuccessfulScan"] = lib.LastScanAt
				}
			}
			out = append(out, item)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})
	r.Post("/external-source/{source}/{action}", func(w http.ResponseWriter, r *http.Request) {
		cfg, e := externaladapter.Sources()
		if e != nil {
			http.Error(w, "Configuration unavailable", 503)
			return
		}
		id := chi.URLParam(r, "source")
		action := chi.URLParam(r, "action")
		uri := ""
		for _, s := range cfg.Sources {
			if s.ID == id && s.Enabled {
				uri = s.Type + "://" + s.ID
			}
		}
		if uri == "" {
			http.NotFound(w, r)
			return
		}
		props := api.ds.Property(r.Context())
		switch action {
		case "enable", "disable":
			value := "false"
			if action == "disable" {
				value = "true"
			}
			e = props.Put("external.disabled."+id, value)
		case "test":
			st, err := storage.For(uri)
			if err == nil {
				var f storage.MusicFS
				f, err = st.FS()
				if err == nil {
					_, err = fs.Stat(f, ".")
				}
			}
			status := "Online"
			if err != nil {
				status = "Offline"
			}
			e = props.Put("external.status."+id, status)
			if e == nil {
				e = props.Put("external.lastcheck."+id, time.Now().UTC().Format(time.RFC3339))
			}
			if err != nil {
				http.Error(w, "Source unavailable; existing catalog preserved", 503)
				return
			}
		case "scan", "remove":
			libs, err := api.ds.Library(r.Context()).GetAll()
			if err != nil {
				http.Error(w, "Catalog unavailable", 503)
				return
			}
			var target *model.Library
			for i := range libs {
				if libs[i].Path == uri {
					target = &libs[i]
					break
				}
			}
			if target == nil {
				http.NotFound(w, r)
				return
			}
			if action == "scan" {
				scanner, ok := api.libs.(interface {
					ScanLibrary(context.Context, int) error
				})
				if !ok {
					http.Error(w, "Scanner unavailable", 503)
					return
				}
				e = scanner.ScanLibrary(r.Context(), target.ID)
			} else {
				if r.Header.Get("X-ND-Confirm-Remove") != id {
					http.Error(w, "Explicit confirmation required", 400)
					return
				}
				e = api.libs.NewRepository(r.Context()).(rest.Persistable).Delete(strconv.Itoa(target.ID))
			}
		case "add":
			// Native library service validates root connectivity and does not automatically grant member access.
			lib := model.Library{Name: id, Path: uri, DefaultNewUsers: false}
			repo := api.libs.NewRepository(r.Context())
			_, e = repo.(rest.Persistable).Save(&lib)
		default:
			http.NotFound(w, r)
			return
		}
		if e != nil {
			http.Error(w, "Source action failed", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		log.Info(r.Context(), "External source action completed", "sourceId", id, "action", action)
		w.Write([]byte(`{"ok":true}`))
	})
}
