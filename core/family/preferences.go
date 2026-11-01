package family

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type Preferences struct {
	Widgets          []string `json:"widgets"`
	ForgottenEnabled bool     `json:"forgottenEnabled"`
	ForgottenDays    int      `json:"forgottenDays"`
	ForgottenMode    string   `json:"forgottenMode"`
}

func DefaultPreferences() Preferences {
	return Preferences{Widgets: []string{"new", "shelves", "letters", "capsules", "requests"}, ForgottenDays: 180, ForgottenMode: "long"}
}
func (s Store) Preferences(ctx context.Context) (Preferences, error) {
	u, e := actor(ctx)
	p := DefaultPreferences()
	if e != nil {
		return p, e
	}
	var body string
	e = s.DB.QueryRowContext(ctx, `SELECT body FROM family_preference WHERE user_id=?`, u.ID).Scan(&body)
	if errors.Is(e, sql.ErrNoRows) {
		return p, nil
	}
	if e != nil {
		return p, e
	}
	e = json.Unmarshal([]byte(body), &p)
	if p.Widgets == nil {
		p.Widgets = []string{}
	}
	return p, e
}
func (s Store) SetPreferences(ctx context.Context, p Preferences) error {
	u, e := actor(ctx)
	if e != nil {
		return e
	}
	if p.ForgottenDays < 7 || p.ForgottenDays > 3650 || len(p.Widgets) > 7 {
		return ErrInvalid
	}
	if p.ForgottenMode != "long" && p.ForgottenMode != "favorites" && p.ForgottenMode != "surprise" {
		return ErrInvalid
	}
	if p.Widgets == nil {
		p.Widgets = []string{}
	}
	seen := map[string]bool{}
	for _, k := range p.Widgets {
		if seen[k] || (k != "continue" && k != "shelves" && k != "new" && k != "forgotten" && k != "requests" && k != "letters" && k != "capsules") {
			return ErrInvalid
		}
		seen[k] = true
	}
	b, _ := json.Marshal(p)
	_, e = s.DB.ExecContext(ctx, `INSERT INTO family_preference(user_id,body) VALUES(?,?) ON CONFLICT(user_id) DO UPDATE SET body=excluded.body`, u.ID, string(b))
	return e
}
func (s Store) Dismiss(ctx context.Context, album string) error {
	u, e := actor(ctx)
	if e != nil {
		return e
	}
	if _, e = s.Resolve(ctx, "album", album); e != nil {
		return ErrDenied
	}
	_, e = s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO family_dismissal(user_id,album_id) VALUES(?,?)`, u.ID, album)
	return e
}
