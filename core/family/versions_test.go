package family

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/model/request"
)

func collectorFixture(t *testing.T) (Store, context.Context, context.Context) {
	s, o, w, _ := fixture(t)
	migration, e := os.ReadFile("../../db/migrations/20261031160000_family_collectors.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(strings.Split(string(migration), "-- +goose Down")[0]); e != nil {
		t.Fatal(e)
	}
	return s, o, w
}
func TestVersionsStrongGroupingPreferenceIsolationAndRevocation(t *testing.T) {
	s, o, w := collectorFixture(t)
	if _, e := s.DB.Exec(`CREATE TABLE album(id TEXT PRIMARY KEY,library_id INTEGER);CREATE TABLE user_library(user_id TEXT,library_id INTEGER);INSERT INTO album VALUES('album-one',1),('album-two',2),('unrelated',1);INSERT INTO user_library VALUES('member',1),('member',2);`); e != nil {
		t.Fatal(e)
	}
	allowed := true
	const group = "11111111-2222-3333-4444-555555555555"
	v := Versions{DB: s.DB, Album: func(ctx context.Context, key string) (VersionAlbum, error) {
		u, _ := request.UserFrom(ctx)
		if key == "album-two" && !allowed && !u.IsAdmin {
			return VersionAlbum{}, ErrDenied
		}
		g := group
		if key == "unrelated" {
			g = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
		}
		return VersionAlbum{Reference: Reference{Kind: "album", ID: key, Title: "Same title is not proof", Available: false}, ReleaseGroup: g}, nil
	}}
	if _, e := v.Create(w, []string{"album-one", "album-two"}, "musicbrainz", false); !errors.Is(e, ErrDenied) {
		t.Fatal("member created catalog relationship", e)
	}
	if _, e := v.Create(o, []string{"album-one", "unrelated"}, "musicbrainz", false); !errors.Is(e, ErrInvalid) {
		t.Fatal("different release groups merged", e)
	}
	if _, e := v.Create(o, []string{"album-one", "album-two"}, "confirmed", false); !errors.Is(e, ErrInvalid) {
		t.Fatal("manual confirmation skipped", e)
	}
	x, e := v.Create(o, []string{"album-one", "album-two"}, "musicbrainz", false)
	if e != nil || len(x.Members) != 2 {
		t.Fatal(x, e)
	}
	repeat, e := v.Create(o, []string{"album-one", "album-two"}, "musicbrainz", false)
	if e != nil || repeat.ID != x.ID {
		t.Fatal("discovery duplicated group", repeat, e)
	}
	if e = v.Prefer(w, x.ID, "album-two"); e != nil {
		t.Fatal(e)
	}
	member, e := v.Get(w, x.ID)
	if e != nil || member.Preferred != "album-two" || member.CanEdit {
		t.Fatal(member, e)
	}
	owner, e := v.Get(o, x.ID)
	if e != nil || owner.Preferred != "" {
		t.Fatal("personal preference leaked", owner, e)
	}
	allowed = false
	if _, e = s.DB.Exec(`DELETE FROM user_library WHERE user_id='member' AND library_id=2`); e != nil {
		t.Fatal(e)
	}
	member, e = v.Get(w, x.ID)
	if e != nil || len(member.Members) != 1 || member.Preferred != "" {
		t.Fatal("revoked version metadata leaked", member, e)
	}
	if e = v.Prefer(w, x.ID, "album-two"); !errors.Is(e, ErrDenied) {
		t.Fatal("revoked preference accepted", e)
	}
	xs, e := v.List(w, false, 0)
	if e != nil || len(xs) != 1 {
		t.Fatal(xs, e)
	}
	if _, e = s.DB.Exec(`DELETE FROM user_library WHERE user_id='member'`); e != nil {
		t.Fatal(e)
	}
	xs, e = v.List(w, false, 0)
	if e != nil || len(xs) != 0 {
		t.Fatal("group leaked before pagination", xs, e)
	}
	if _, e = v.Archive(w, x.ID, x.Revision, true); !errors.Is(e, ErrDenied) {
		t.Fatal("member archived group", e)
	}
	owner, e = v.Get(o, x.ID)
	if e != nil {
		t.Fatal(e)
	}
	owner, e = v.Archive(o, x.ID, owner.Revision, true)
	if e != nil || !owner.Archived {
		t.Fatal(owner, e)
	}
	owner, e = v.Archive(o, x.ID, owner.Revision, false)
	if e != nil || owner.Archived {
		t.Fatal(owner, e)
	}
	// No source/catalog mutation is part of grouping.
	var count int
	if e = s.DB.QueryRow(`SELECT count(*) FROM album`).Scan(&count); e != nil || count != 3 {
		t.Fatal("album identities changed", count, e)
	}
}
