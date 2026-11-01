-- +goose Up
CREATE TABLE family_child_profile (
 user_id TEXT PRIMARY KEY REFERENCES user(id) ON DELETE CASCADE,
 parent_id TEXT NOT NULL REFERENCES user(id) ON DELETE RESTRICT,
 body TEXT NOT NULL CHECK(json_valid(body) AND length(CAST(body AS BLOB))<=4096),
 revision INTEGER NOT NULL DEFAULT 1,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 CHECK(user_id<>parent_id)
);
CREATE INDEX family_child_parent ON family_child_profile(parent_id,user_id);
-- +goose StatementBegin
CREATE TRIGGER family_child_role_insert BEFORE INSERT ON family_child_profile
WHEN (SELECT is_admin FROM user WHERE id=NEW.user_id)<>0 OR (SELECT is_admin FROM user WHERE id=NEW.parent_id)<>1 OR (SELECT count(*) FROM family_child_profile)>=10
BEGIN SELECT RAISE(ABORT,'invalid protected family role'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER family_child_role_update BEFORE UPDATE OF is_admin ON user
WHEN (NEW.is_admin=1 AND EXISTS(SELECT 1 FROM family_child_profile WHERE user_id=NEW.id)) OR (NEW.is_admin=0 AND EXISTS(SELECT 1 FROM family_child_profile WHERE parent_id=NEW.id))
BEGIN SELECT RAISE(ABORT,'protected family role'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER family_child_playlist_insert BEFORE INSERT ON playlist
WHEN NEW.public=1 AND EXISTS(SELECT 1 FROM family_child_profile WHERE user_id=NEW.owner_id)
BEGIN SELECT RAISE(ABORT,'protected playlists stay private'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER family_child_playlist_update BEFORE UPDATE OF public,owner_id ON playlist
WHEN NEW.public=1 AND EXISTS(SELECT 1 FROM family_child_profile WHERE user_id=NEW.owner_id)
BEGIN SELECT RAISE(ABORT,'protected playlists stay private'); END;
-- +goose StatementEnd
CREATE TABLE family_legacy_export (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL UNIQUE REFERENCES user(id) ON DELETE CASCADE,
 options TEXT NOT NULL CHECK(json_valid(options) AND length(CAST(options AS BLOB))<=65536),
 created_at TEXT NOT NULL,
 expires_at TEXT NOT NULL
);
CREATE INDEX family_legacy_owner ON family_legacy_export(user_id,expires_at);
-- +goose StatementBegin
CREATE TRIGGER family_legacy_limit BEFORE INSERT ON family_legacy_export
WHEN (SELECT count(*) FROM family_legacy_export)>=256 AND NOT EXISTS(SELECT 1 FROM family_legacy_export WHERE user_id=NEW.user_id)
BEGIN SELECT RAISE(ABORT,'archive descriptor quota'); END;
-- +goose StatementEnd
-- +goose Down
DROP TRIGGER family_legacy_limit;
DROP TABLE family_legacy_export;
DROP TRIGGER family_child_playlist_update;
DROP TRIGGER family_child_playlist_insert;
DROP TRIGGER family_child_role_update;
DROP TRIGGER family_child_role_insert;
DROP TABLE family_child_profile;
