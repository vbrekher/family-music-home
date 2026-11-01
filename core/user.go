package core

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/model"
)

// PluginUnloader defines the interface for unloading disabled plugins.
// This is satisfied by plugins.Manager but defined here to avoid import cycles.
type PluginUnloader interface {
	UnloadDisabledPlugins(ctx context.Context)
}

// User provides business logic for user management with plugin coordination.
type User interface {
	NewRepository(ctx context.Context) rest.Repository
}

type userService struct {
	ds            model.DataStore
	pluginManager PluginUnloader
}

// NewUser creates a new User service
func NewUser(ds model.DataStore, pluginManager PluginUnloader) User {
	return &userService{
		ds:            ds,
		pluginManager: pluginManager,
	}
}

// NewRepository returns a REST repository wrapper for user operations.
// The wrapper intercepts Delete operations to coordinate plugin unloading.
func (s *userService) NewRepository(ctx context.Context) rest.Repository {
	repo := s.ds.User(ctx)
	wrapper := &userRepositoryWrapper{
		ctx:            ctx,
		UserRepository: repo,
		pluginManager:  s.pluginManager,
	}
	return wrapper
}

type userRepositoryWrapper struct {
	model.UserRepository
	ctx           context.Context
	pluginManager PluginUnloader
}

// Save implements rest.Persistable by delegating to the underlying repository.
var familyUserLock sync.Mutex

func (r *userRepositoryWrapper) Save(entity any) (string, error) {
	if os.Getenv("BUNNY_FAMILY_MODE") == "true" {
		familyUserLock.Lock()
		defer familyUserLock.Unlock()
		count, err := r.UserRepository.CountAll()
		if err != nil {
			return "", err
		}
		if count >= 2 {
			return "", &rest.ValidationError{Errors: map[string]string{"userName": "Family service permits exactly two accounts"}}
		}
		if user, ok := entity.(*model.User); ok && (user.IsAdmin || user.UserName != familyMemberName()) {
			return "", &rest.ValidationError{Errors: map[string]string{"isAdmin": "The second account must be the configured family member"}}
		}
	}
	return r.UserRepository.(rest.Persistable).Save(entity)
}

// Update implements rest.Persistable by delegating to the underlying repository.
func (r *userRepositoryWrapper) Update(id string, entity any, cols ...string) error {
	if os.Getenv("BUNNY_FAMILY_MODE") == "true" {
		existing, err := r.UserRepository.Get(id)
		if err != nil {
			return err
		}
		if user, ok := entity.(*model.User); ok && (user.IsAdmin != existing.IsAdmin || user.UserName != existing.UserName) {
			return &rest.ValidationError{Errors: map[string]string{"userName": "Family account names and roles are fixed"}}
		}
	}
	return r.UserRepository.(rest.Persistable).Update(id, entity, cols...)
}

// Delete implements rest.Persistable and coordinates plugin unloading.
func (r *userRepositoryWrapper) Delete(id string) error {
	if os.Getenv("BUNNY_FAMILY_MODE") == "true" {
		existing, err := r.UserRepository.Get(id)
		if err != nil {
			return err
		}
		if existing.IsAdmin || existing.FamilyChild == nil {
			return fmt.Errorf("%w: permanent family accounts cannot be deleted", rest.ErrPermissionDenied)
		}
	}
	// The underlying repository Delete handles the database cleanup
	// including calling cleanupPluginUserReferences
	err := r.UserRepository.(rest.Persistable).Delete(id)
	if err != nil {
		return err
	}

	// After successful deletion, check if any plugins were auto-disabled
	// and need to be unloaded from memory
	r.pluginManager.UnloadDisabledPlugins(r.ctx)

	return nil
}

func familyMemberName() string {
	if name := os.Getenv("BUNNY_MEMBER_USERNAME"); name != "" {
		return name
	}
	return "family-member"
}
