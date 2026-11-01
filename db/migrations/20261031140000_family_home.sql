-- +goose Up
CREATE TABLE family_entry (
 id TEXT PRIMARY KEY,
 kind TEXT NOT NULL CHECK(kind IN ('shelf','capsule','letter','request')),
 owner_id TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
 title TEXT NOT NULL,
 body TEXT NOT NULL CHECK(json_valid(body)),
 archived INTEGER NOT NULL DEFAULT 0 CHECK(archived IN (0,1)),
 revision INTEGER NOT NULL DEFAULT 1,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE INDEX family_entry_owner_kind ON family_entry(owner_id,kind,archived,updated_at);
CREATE TABLE family_access (
 entry_id TEXT NOT NULL REFERENCES family_entry(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
 PRIMARY KEY(entry_id,user_id)
);
CREATE INDEX family_access_reader ON family_access(user_id,entry_id);
CREATE TABLE family_preference (
 user_id TEXT PRIMARY KEY REFERENCES user(id) ON DELETE CASCADE,
 body TEXT NOT NULL CHECK(json_valid(body))
);
CREATE TABLE family_dismissal (
 user_id TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
 album_id TEXT NOT NULL,
 PRIMARY KEY(user_id,album_id)
);
-- +goose Down
DROP TABLE family_dismissal;
DROP TABLE family_preference;
DROP TABLE family_access;
DROP TABLE family_entry;
