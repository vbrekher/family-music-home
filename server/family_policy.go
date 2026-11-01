package server

import (
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"net/http"
	"strings"
)

// ProtectedFamilyRoute is an allowlist, not a UI permission hint. Unknown routes
// remain unavailable to protected accounts, including alternate native clients.
func ProtectedFamilyRoute(u model.User, path, method string) bool {
	p := u.FamilyChild
	if p == nil {
		return true
	}
	if u.IsAdmin {
		return false
	}
	if i := strings.Index(path, "/api/"); i >= 0 {
		path = strings.Trim(path[i+5:], "/")
		parts := strings.Split(path, "/")
		if path == "family/me" || path == "family/kids" {
			return method == http.MethodGet
		}
		if !p.Enabled {
			return false
		}
		if parts[0] == "family" && len(parts) > 1 {
			return parts[1] != "children" && parts[1] != "doctor"
		}
		if path == "personal/theme" {
			return method == http.MethodGet || method == http.MethodPut
		}
		switch parts[0] {
		case "song", "album", "artist", "genre", "tag":
			return method == http.MethodGet && len(parts) <= 2
		case "queue":
			return method == http.MethodGet || method == http.MethodPost || method == http.MethodPut
		case "playlist", "playlistTrack":
			return true // canonical repository enforces ownership and library grants
		case "keepalive":
			return method == http.MethodGet
		}
		return false
	}
	if strings.Contains(path, "/rest/") {
		if !p.Enabled {
			return false
		}
		name := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(path[strings.LastIndex(path, "/")+1:], ".view"), ".json"))
		if name == "gettranscodestream" {
			// The canonical handler rechecks current library access and signed
			// media parameters; the WebUI uses this read-only playback endpoint.
			return method == http.MethodGet || method == http.MethodHead
		}
		if name == "gettranscodedecision" {
			return method == http.MethodPost
		}
		switch name {
		case "ping", "getlicense", "getopensubsonicextensions", "getmusicfolders", "getmusicdirectory", "getindexes", "getartists", "getartist", "getalbum", "getsong", "getalbumlist", "getalbumlist2", "getrandomsongs", "getsongsbygenre", "getgenres", "search", "search2", "search3", "getstarred", "getstarred2", "star", "unstar", "setrating", "stream", "download", "getcoverart", "scrobble", "reportplayback", "getplayqueue", "saveplayqueue", "getplaylists", "getplaylist", "createplaylist", "updateplaylist", "deleteplaylist", "getbookmarks", "createbookmark", "deletebookmark":
			return true
		}
		return false
	}
	return false // events, external integrations and additional protocols fail closed
}
func EnforceProtectedFamily(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, ok := request.UserFrom(r.Context()); ok && !ProtectedFamilyRoute(u, r.URL.Path, r.Method) {
			w.Header().Set("Cache-Control", "private, no-store")
			http.Error(w, "This account is restricted by family policy", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
