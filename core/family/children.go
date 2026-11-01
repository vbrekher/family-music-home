package family

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/navidrome/navidrome/model"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

type ChildProfile struct {
	UserID     string            `json:"userId"`
	UserName   string            `json:"userName"`
	Name       string            `json:"name"`
	Policy     model.FamilyChild `json:"policy"`
	LibraryIDs []int             `json:"libraryIds"`
	Revision   int               `json:"revision"`
}

func ValidateChildName(username, name string) error {
	if len(username) < 1 || len(username) > 64 || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 80 || strings.TrimSpace(name) != name {
		return ErrInvalid
	}
	for _, r := range username {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' && r != '-' && r != '.' {
			return ErrInvalid
		}
	}
	if strings.EqualFold(username, os.Getenv("BUNNY_OWNER_USERNAME")) || strings.EqualFold(username, os.Getenv("BUNNY_MEMBER_USERNAME")) {
		return ErrDenied
	}
	return nil
}
func ValidateChildPolicy(p model.FamilyChild, libraries []int) error {
	if p.Interface != "kids" && p.Interface != "full" || p.Density != "large" && p.Density != "comfortable" || len(libraries) > 20 || len(p.StoryLibraries) > 20 {
		return ErrInvalid
	}
	seen := map[int]bool{}
	for _, id := range libraries {
		if id < 1 || seen[id] {
			return ErrInvalid
		}
		seen[id] = true
	}
	story := map[int]bool{}
	for _, id := range p.StoryLibraries {
		if !seen[id] || story[id] {
			return ErrInvalid
		}
		story[id] = true
	}
	return nil
}

// ChildReaderAllowed supplements explicit sharing; a parent must opt in before
// other adults can make family material visible to the protected account.
func ChildReaderAllowed(u model.User, owner string) bool {
	return u.FamilyChild == nil || u.FamilyChild.Enabled && (u.ID == owner || u.FamilyChild.AllowFamilyMessages || u.FamilyChild.ParentID == owner)
}
func ChildProfiles(ctx context.Context, db *sql.DB) ([]ChildProfile, error) {
	u, e := actor(ctx)
	if e != nil || !u.IsAdmin || u.FamilyChild != nil {
		return nil, ErrDenied
	}
	rows, e := db.QueryContext(ctx, `SELECT p.user_id,u.user_name,u.name,p.body,p.revision,p.parent_id FROM family_child_profile p JOIN user u ON u.id=p.user_id WHERE p.parent_id=? ORDER BY p.created_at,p.user_id`, u.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []ChildProfile{}
	for rows.Next() {
		var p ChildProfile
		var raw string
		if e = rows.Scan(&p.UserID, &p.UserName, &p.Name, &raw, &p.Revision, &p.Policy.ParentID); e != nil {
			return nil, e
		}
		parent := p.Policy.ParentID
		if e = json.Unmarshal([]byte(raw), &p.Policy); e != nil {
			return nil, e
		}
		p.Policy.ParentID = parent
		out = append(out, p)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	return out, nil
}
