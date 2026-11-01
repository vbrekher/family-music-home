-- +goose Up
CREATE TABLE family_version_group (
 id TEXT PRIMARY KEY,
 owner_id TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
 basis TEXT NOT NULL CHECK(basis IN ('musicbrainz','confirmed')),
 release_group TEXT NOT NULL DEFAULT '',
 archived INTEGER NOT NULL DEFAULT 0 CHECK(archived IN (0,1)),
 revision INTEGER NOT NULL DEFAULT 1,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX family_version_release_group ON family_version_group(release_group) WHERE basis='musicbrainz';
CREATE TABLE family_version_member (
 group_id TEXT NOT NULL REFERENCES family_version_group(id) ON DELETE CASCADE,
 album_id TEXT NOT NULL,
 PRIMARY KEY(group_id,album_id)
);
CREATE INDEX family_version_album ON family_version_member(album_id,group_id);
CREATE TABLE family_version_preference (
 user_id TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
 group_id TEXT NOT NULL REFERENCES family_version_group(id) ON DELETE CASCADE,
 album_id TEXT NOT NULL,
 PRIMARY KEY(user_id,group_id)
);
CREATE TABLE family_doctor_job (
 id TEXT PRIMARY KEY,
 owner_id TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
 mode TEXT NOT NULL CHECK(mode IN ('metadata','deep')),
 state TEXT NOT NULL CHECK(state IN ('queued','active','paused','cancelled','completed','failed')),
 settings TEXT NOT NULL,
 cursor TEXT NOT NULL DEFAULT '',
 processed INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX family_doctor_one_active ON family_doctor_job((1)) WHERE state IN ('queued','active');
CREATE INDEX family_doctor_owner ON family_doctor_job(owner_id,created_at);
CREATE TABLE family_doctor_budget (
 job_id TEXT NOT NULL REFERENCES family_doctor_job(id) ON DELETE CASCADE,
 library_id INTEGER NOT NULL,
 limit_bytes INTEGER NOT NULL CHECK(limit_bytes>=0),
 reserved_bytes INTEGER NOT NULL DEFAULT 0 CHECK(reserved_bytes>=0),
 PRIMARY KEY(job_id,library_id)
);
CREATE TABLE family_doctor_item (
 job_id TEXT NOT NULL REFERENCES family_doctor_job(id) ON DELETE CASCADE,
 track_id TEXT NOT NULL,
 library_id INTEGER NOT NULL,
 snapshot TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('queued','leased','fetching','downloaded','completed','unavailable','changed','budget','failed','cancelled')),
 lease TEXT NOT NULL DEFAULT '',
 lease_until INTEGER NOT NULL DEFAULT 0,
 attempts INTEGER NOT NULL DEFAULT 0,
 bytes_read INTEGER NOT NULL DEFAULT 0,
 sha256 TEXT NOT NULL DEFAULT '',
 result TEXT NOT NULL DEFAULT '{}',
 updated_at TEXT NOT NULL,
 PRIMARY KEY(job_id,track_id)
);
CREATE INDEX family_doctor_queue ON family_doctor_item(job_id,state,lease_until);
CREATE INDEX family_doctor_track ON family_doctor_item(track_id,updated_at);
CREATE TABLE family_doctor_album_audit (
 job_id TEXT NOT NULL REFERENCES family_doctor_job(id) ON DELETE CASCADE,
 album_id TEXT NOT NULL,
 PRIMARY KEY(job_id,album_id)
);
CREATE TABLE family_doctor_finding (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 job_id TEXT NOT NULL REFERENCES family_doctor_job(id) ON DELETE CASCADE,
 track_id TEXT NOT NULL,
 library_id INTEGER NOT NULL,
 severity TEXT NOT NULL CHECK(severity IN ('info','warning','error')),
 code TEXT NOT NULL,
 evidence TEXT NOT NULL,
 created_at TEXT NOT NULL
);
CREATE INDEX family_doctor_findings ON family_doctor_finding(job_id,severity,id);
CREATE TABLE family_doctor_analysis (
 track_id TEXT PRIMARY KEY,
 library_id INTEGER NOT NULL,
 catalog_version TEXT NOT NULL,
 sha256 TEXT NOT NULL,
 result TEXT NOT NULL,
 verified_at TEXT NOT NULL
);
-- +goose Down
DROP TABLE family_doctor_analysis;
DROP TABLE family_doctor_album_audit;
DROP TABLE family_doctor_finding;
DROP TABLE family_doctor_item;
DROP TABLE family_doctor_budget;
DROP TABLE family_doctor_job;
DROP TABLE family_version_preference;
DROP TABLE family_version_member;
DROP TABLE family_version_group;
