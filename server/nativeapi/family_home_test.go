package nativeapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/tests"
)

func TestFamilyPassportDoesNotInferProvenanceOrExposeOrigin(t *testing.T) {
	repo := tests.CreateMockMediaFileRepo()
	repo.SetData(model.MediaFiles{{ID: "track", Title: "Unverified FLAC", Suffix: "flac", SampleRate: 192000, LibraryID: 1, LibraryPath: "bunny://private-user-secret", Path: "private/path-secret"}})
	api := &Router{ds: &tests.MockDataStore{MockedMediaFile: repo}}
	router := chi.NewRouter()
	router.Get("/passport/{id}", api.familyPassport)
	r := httptest.NewRequest("GET", "/passport/track", nil).WithContext(request.WithUser(context.Background(), model.User{ID: "user"}))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var v map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
		t.Fatal(e)
	}
	if v["codec"] != "Unknown" || v["compression"] != "Unknown" || v["provenance"] != "Unknown" || (v["integrity"] != "Not analyzed" && v["integrity"] != "Analysis temporarily unavailable") {
		t.Fatal("unverified facts fabricated", v)
	}
	if strings.Contains(w.Body.String(), "secret") || v["path"] != nil || v["libraryPath"] != nil {
		t.Fatal("origin exposed")
	}
	r = httptest.NewRequest("GET", "/passport/missing", nil).WithContext(r.Context())
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal("missing track not hidden", w.Code)
	}
}
