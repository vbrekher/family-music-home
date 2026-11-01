-- +goose Up
CREATE TABLE family_delivery (
 entry_id TEXT NOT NULL REFERENCES family_entry(id) ON DELETE CASCADE,
 recipient_id TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
 due_at INTEGER NOT NULL,
 send_key TEXT NOT NULL,
 recurring INTEGER NOT NULL DEFAULT 0 CHECK(recurring IN (0,1)),
 delivered_at INTEGER,
 read_at INTEGER,
 PRIMARY KEY(entry_id,recipient_id,due_at,send_key)
);
CREATE INDEX family_delivery_due ON family_delivery(delivered_at,due_at);
CREATE INDEX family_delivery_inbox ON family_delivery(recipient_id,delivered_at,read_at);
CREATE TABLE family_action (
 entry_id TEXT NOT NULL REFERENCES family_entry(id) ON DELETE CASCADE,
 action_key TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(entry_id,action_key)
);
CREATE TABLE family_inbox_state (
 entry_id TEXT NOT NULL REFERENCES family_entry(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
 archived INTEGER NOT NULL DEFAULT 0 CHECK(archived IN (0,1)),
 PRIMARY KEY(entry_id,user_id)
);
CREATE TABLE family_request_history (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 entry_id TEXT NOT NULL REFERENCES family_entry(id) ON DELETE CASCADE,
 item_id TEXT NOT NULL,
 actor_id TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
 state TEXT NOT NULL,
 comment TEXT NOT NULL,
 created_at TEXT NOT NULL
);
CREATE INDEX family_request_history_entry ON family_request_history(entry_id,id);
CREATE TABLE family_notification (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
 entry_id TEXT NOT NULL REFERENCES family_entry(id) ON DELETE CASCADE,
 message TEXT NOT NULL,
 created_at TEXT NOT NULL,
 read_at TEXT
);
CREATE INDEX family_notification_inbox ON family_notification(user_id,read_at,created_at);
CREATE TABLE family_attachment (
 id TEXT PRIMARY KEY,
 entry_id TEXT NOT NULL REFERENCES family_entry(id) ON DELETE CASCADE,
 owner_id TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
 mime TEXT NOT NULL,
 size INTEGER NOT NULL CHECK(size>0 AND size<=5242880),
 sha256 TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('pending','ready')),
 hidden INTEGER NOT NULL DEFAULT 0 CHECK(hidden IN (0,1)),
 created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX family_attachment_content ON family_attachment(entry_id,sha256);
CREATE INDEX family_attachment_entry ON family_attachment(entry_id,state);
CREATE INDEX family_attachment_quota ON family_attachment(owner_id,size);
-- +goose Down
DROP TABLE family_attachment;
DROP TABLE family_notification;
DROP TABLE family_request_history;
DROP TABLE family_inbox_state;
DROP TABLE family_action;
DROP TABLE family_delivery;
