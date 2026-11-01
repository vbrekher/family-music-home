package family

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
)

func fixture(t *testing.T) (Store, context.Context, context.Context, *bool) {
	t.Helper()
	d, e := sql.Open("sqlite3", filepath.Join(t.TempDir(), "family.db")+"?_foreign_keys=on")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { d.Close() })
	_, e = d.Exec(`CREATE TABLE user(id TEXT PRIMARY KEY); INSERT INTO user VALUES('owner'),('member'),('child');`)
	if e != nil {
		t.Fatal(e)
	}
	migration, e := os.ReadFile("../../db/migrations/20261031140000_family_home.sql")
	if e != nil {
		t.Fatal(e)
	}
	_, e = d.Exec(strings.Split(string(migration), "-- +goose Down")[0])
	if e != nil {
		t.Fatal(e)
	}
	interaction, e := os.ReadFile("../../db/migrations/20261031150000_family_interactions.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = d.Exec(strings.Split(string(interaction), "-- +goose Down")[0]); e != nil {
		t.Fatal(e)
	}
	allowed := true
	s := Store{DB: d, IsAdministrator: func(ctx context.Context, id string) bool { return id == "owner" }, Resolve: func(ctx context.Context, kind, id string) (Reference, error) {
		if !allowed || id == "secret" || kind != "track" && kind != "album" && kind != "playlist" {
			return Reference{}, ErrDenied
		}
		return Reference{Kind: kind, ID: id, Title: "Snapshot title", Available: false}, nil
	}, UserExists: func(ctx context.Context, id string) bool { return id == "member" || id == "owner" || id == "child" }}
	return s, request.WithUser(context.Background(), model.User{ID: "owner", IsAdmin: true}), request.WithUser(context.Background(), model.User{ID: "member"}), &allowed
}
func TestPrivateSharingAndConflicts(t *testing.T) {
	s, owner, member, allowed := fixture(t)
	x, e := s.Save(member, Entry{Kind: "shelf", Title: "Member private", Body: Content{References: []Reference{{Kind: "track", ID: "song"}}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(owner, x.ID); !errors.Is(e, ErrDenied) {
		t.Fatal("admin read another adult's private shelf", e)
	}
	x.SharedWith = []string{"owner"}
	x, e = s.Save(member, x)
	if e != nil {
		t.Fatal(e)
	}
	read, e := s.Get(owner, x.ID)
	if e != nil || len(read.Body.References) != 1 || read.Body.References[0].Title != "Snapshot title" || read.Body.References[0].Available {
		t.Fatal("offline snapshot lost", read, e)
	}
	read.Title = "takeover"
	if _, e = s.Save(owner, read); !errors.Is(e, ErrConflict) {
		t.Fatal("recipient modified shelf", e)
	}
	stale := x
	x.Title = "new title"
	x, e = s.Save(member, x)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Save(member, stale); !errors.Is(e, ErrConflict) {
		t.Fatal("stale update accepted", e)
	}
	*allowed = false
	read, e = s.Get(owner, x.ID)
	if e != nil || read.Body.References[0].ID != "" || read.Body.References[0].Title != "Unavailable item" {
		t.Fatal("revoked metadata leaked", read, e)
	}
}
func TestValidationAndAccountIsolation(t *testing.T) {
	s, o, w, _ := fixture(t)
	for _, x := range []Entry{{Kind: "shelf", Title: ""}, {Kind: "shelf", Title: "x", SharedWith: []string{"outsider"}}, {Kind: "shelf", Title: "x", Body: Content{References: []Reference{{Kind: "track", ID: "secret"}}}}} {
		if _, e := s.Save(o, x); e == nil {
			t.Fatal("invalid shelf accepted", x)
		}
	}
	x, e := s.Save(o, Entry{Kind: "shelf", Title: "unicode 🐰", Archived: true})
	if e != nil {
		t.Fatal(e)
	}
	xs, e := s.List(o, "shelf", false, 0)
	if e != nil || len(xs) != 0 {
		t.Fatal(xs, e)
	}
	xs, e = s.List(o, "shelf", true, 0)
	if e != nil || len(xs) != 1 {
		t.Fatal(xs, e)
	}
	x.Archived = false
	if _, e = s.Save(o, x); e != nil {
		t.Fatal(e)
	}
	xs, e = s.List(w, "shelf", false, 0)
	if e != nil || len(xs) != 0 {
		t.Fatal("private list leak", xs, e)
	}
	p := DefaultPreferences()
	p.Widgets = []string{"shelves"}
	if e = s.SetPreferences(o, p); e != nil {
		t.Fatal(e)
	}
	other, e := s.Preferences(w)
	if e != nil || len(other.Widgets) != len(DefaultPreferences().Widgets) {
		t.Fatal("preference leak", other, e)
	}
	p.Widgets = []string{"shelves", "shelves"}
	if e = s.SetPreferences(o, p); !errors.Is(e, ErrInvalid) {
		t.Fatal("duplicate widget accepted", e)
	}
	if e = s.Dismiss(o, "secret"); e == nil {
		t.Fatal("inaccessible album dismissal accepted")
	}
}
