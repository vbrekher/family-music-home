package nativeapi

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/model/request"
	"net/http"
	"regexp"
)

var themeName = regexp.MustCompile(`^(?:[A-Za-z][A-Za-z0-9]*Theme|AUTO_THEME_ID)$`)

func (api *Router) addPersonalThemeRoute(r chi.Router) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		user, ok := request.UserFrom(r.Context())
		if !ok {
			http.Error(w, "Unauthorized", 401)
			return
		}
		key := "personal.theme." + user.ID
		repo := api.ds.Property(r.Context())
		if r.Method == "PUT" {
			var data struct {
				Theme string `json:"theme"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 256)).Decode(&data) != nil || !themeName.MatchString(data.Theme) {
				http.Error(w, "Invalid theme", 400)
				return
			}
			if e := repo.Put(key, data.Theme); e != nil {
				http.Error(w, "Preference could not be saved", 503)
				return
			}
		}
		value, e := repo.DefaultGet(key, "")
		if e != nil {
			http.Error(w, "Preference unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"theme": value})
	}
	r.Get("/personal/theme", handler)
	r.Put("/personal/theme", handler)
}
