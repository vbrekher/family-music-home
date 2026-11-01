package core

import (
	"context"
	"errors"
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
	"testing"
)

func TestFamilyAccountPolicy(t *testing.T) {
	t.Setenv("BUNNY_FAMILY_MODE", "true")
	users := tests.CreateMockUserRepo()
	for _, u := range []model.User{{ID: "owner", UserName: "admin", IsAdmin: true}, {ID: "member", UserName: "member"}} {
		if err := users.Put(&u); err != nil {
			t.Fatal(err)
		}
	}
	r := userRepositoryWrapper{UserRepository: users, ctx: context.Background()}
	if _, err := r.Save(&model.User{UserName: "third"}); !isFamilyValidation(err) {
		t.Fatalf("third account: %v", err)
	}
	if err := r.Update("member", &model.User{ID: "member", UserName: "member", IsAdmin: true}); !isFamilyValidation(err) {
		t.Fatalf("promotion: %v", err)
	}
	if err := r.Delete("member"); !errors.Is(err, rest.ErrPermissionDenied) {
		t.Fatalf("member deletion: %v", err)
	}
	if err := r.Delete("owner"); !errors.Is(err, rest.ErrPermissionDenied) {
		t.Fatalf("owner deletion: %v", err)
	}
}

func isFamilyValidation(err error) bool {
	var validation *rest.ValidationError
	return errors.As(err, &validation)
}
