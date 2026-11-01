package persistence

import (
	"context"
	"github.com/navidrome/navidrome/model"
	"testing"
)

func TestPasswordEncryptionFailureStopsBeforeAnyDatabaseWrite(t *testing.T) {
	original := encKey
	defer func() { encKey = original }()
	encKey = []byte("invalid")
	r := &userRepository{sqlRepository: sqlRepository{ctx: context.Background()}}
	if e := r.Put(&model.User{ID: "synthetic0001", UserName: "synthetic", NewPassword: "synthetic-test-password"}); e == nil {
		t.Fatal("encryption failure must not reach database writes")
	}
}
