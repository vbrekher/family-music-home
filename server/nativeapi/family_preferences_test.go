package nativeapi

import (
	"bytes"
	"context"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/conf/configtest"
	"github.com/navidrome/navidrome/core/auth"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server"
	"github.com/navidrome/navidrome/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
)

var _ = Describe("Family preferences authorization", func() {
	It("isolates user themes and denies regular users all source administration", func() {
		DeferCleanup(configtest.SetupConfig())
		conf.Server.EnableSharing = false
		ds := &tests.MockDataStore{}
		auth.Init(ds)
		router := server.JWTVerifier(New(ds, nil, nil, nil, tests.NewMockLibraryService(), tests.NewMockUserService(), nil, nil, nil, nil, nil))
		tokens := map[string]string{}
		for _, name := range []string{"admin", "member"} {
			u := model.User{ID: name, UserName: name, IsAdmin: name == "admin", NewPassword: "test-only"}
			Expect(ds.User(context.Background()).Put(&u)).To(Succeed())
			token, err := auth.CreateToken(&u)
			Expect(err).NotTo(HaveOccurred())
			tokens[name] = token
		}
		call := func(user, method, path, body string) *httptest.ResponseRecorder {
			req := createAuthenticatedRequest(method, path, bytes.NewBufferString(body), tokens[user])
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			return w
		}
		Expect(call("admin", "PUT", "/personal/theme", `{"theme":"NordTheme"}`).Code).To(Equal(http.StatusOK))
		Expect(call("member", "PUT", "/personal/theme", `{"theme":"SpotifyTheme"}`).Code).To(Equal(http.StatusOK))
		Expect(call("admin", "GET", "/personal/theme", "").Body.String()).To(ContainSubstring("NordTheme"))
		Expect(call("member", "GET", "/personal/theme", "").Body.String()).To(ContainSubstring("SpotifyTheme"))
		Expect(call("member", "PUT", "/personal/theme", `{"theme":"../../Owner"}`).Code).To(Equal(400))
		for _, path := range []string{"/external-source/approved/test", "/external-source/approved/add", "/external-source/approved/disable", "/external-source/approved/remove"} {
			Expect(call("member", "POST", path, `{}`).Code).To(Equal(403))
		}
		Expect(call("member", "GET", "/external-source", "").Code).To(Equal(403))
	})
})
