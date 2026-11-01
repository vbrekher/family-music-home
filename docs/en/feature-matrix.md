# Feature inventory

This source inventory covers the inherited server, storage, companion integrations and eleven family modules. Local regression, real protocol tests and production acceptance are separate scopes. A physical NAS/phone or personal production rollout is never implied by a unit test.

[Machine-readable matrix](../release/feature-matrix.json) · [Family module matrix](../release/family-feature-matrix.json)

## Family Home Dashboard

[Source](../../server/nativeapi/family_home.go) · [Interface](../../ui/src/family/FamilyHome.jsx) · [Guide](features/home.md)

Each user owns their Home preferences. Shared entries and music remain subject to current library permissions.

Regression sources: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Family Shelves

[Source](../../core/family/store.go) · [Interface](../../ui/src/family/Shelves.jsx) · [Guide](features/shelves.md)

Only the owner edits the shelf. A shared recipient can read allowed content; an administrator does not gain access to another adult’s private shelf.

Regression sources: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Family Music Time Capsule

[Source](../../core/family/store.go) · [Interface](../../ui/src/family/Journal.jsx) · [Guide](features/capsules.md)

Private by default; explicit recipients and current library access are checked again when reading attachments.

Regression sources: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Music Letters

[Source](../../core/family/letters.go) · [Interface](../../ui/src/family/Journal.jsx) · [Guide](features/letters.md)

Drafts are private to their author. Scheduled letters are hidden from recipients until delivery. Recipient access and attachment authorization are enforced on the server.

Regression sources: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Family Music Requests

[Source](../../core/family/requests.go) · [Interface](../../ui/src/family/Requests.jsx) · [Guide](features/requests.md)

Drafts remain private. Submitted requests go to server-derived administrator recipients; regular members cannot perform administrator status transitions.

Regression sources: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Forgotten Albums

[Source](../../server/nativeapi/family_home.go) · [Interface](../../ui/src/family/FamilyHome.jsx) · [Guide](features/forgotten.md)

Suggestions and dismissals are personal and filtered by current library permissions.

Regression sources: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Music Quality Passport

[Source](../../server/nativeapi/family_home.go) · [Interface](../../ui/src/family/Passport.jsx) · [Guide](features/passport.md)

Only currently authorized catalog references resolve. Secret source paths, credentials and remote object URLs are not part of the passport.

Regression sources: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Library Doctor

[Source](../../core/family/doctor_jobs.go) · [Interface](../../ui/src/family/Doctor.jsx) · [Guide](features/doctor.md)

Administrator only. Protected child accounts cannot access diagnostic jobs or diagnostic audio leases.

Regression sources: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Version Collector

[Source](../../core/family/versions.go) · [Interface](../../ui/src/family/Versions.jsx) · [Guide](features/versions.md)

Catalog permissions are rechecked. Preferences are per user and cannot grant access to an otherwise private library.

Regression sources: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Kids Music World

[Source](../../core/family/children.go) · [Interface](../../ui/src/family/Children.jsx) · [Guide](features/kids.md)

No child account is created automatically. Library access and server route guards apply even when calling APIs directly. Sharing with other adults is off unless explicitly enabled by the parent.

Regression sources: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Family Legacy Export

[Source](../../core/family/legacy.go) · [Interface](../../ui/src/family/Legacy.jsx) · [Guide](features/legacy.md)

An administrator cannot export another adult’s private records. Permissions are checked again at download and attachment reads. The link belongs to the preparing account.

Regression sources: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Music catalog, artists and albums

[Source](../../persistence) · [Interface](../../ui/src/album) · [Guide](library.md)

Library grants; current user

Regression sources: [album_repository_test.go](../../persistence/album_repository_test.go), [artist_repository_test.go](../../persistence/artist_repository_test.go), [AlbumDetails.test.jsx](../../ui/src/album/AlbumDetails.test.jsx).

## Genres, search, filters and sorting

[Source](../../server/nativeapi) · [Interface](../../ui/src/artist) · [Guide](library.md)

Current library grants

Regression sources: [genre_repository_test.go](../../persistence/genre_repository_test.go), [sql_search_fts_test.go](../../persistence/sql_search_fts_test.go), [searching_test.go](../../server/subsonic/searching_test.go).

## Web player and queue

[Source](../../server/subsonic) · [Interface](../../ui/src/audioplayer) · [Guide](player.md)

Current library grants and signed stream authorization

Regression sources: [media_retrieval_test.go](../../server/subsonic/media_retrieval_test.go), [playqueue_repository_test.go](../../persistence/playqueue_repository_test.go), [AudioTitle.test.jsx](../../ui/src/audioplayer/AudioTitle.test.jsx).

## Playlists and smart playlists

[Source](../../core/playlists) · [Interface](../../ui/src/playlist) · [Guide](library.md)

Playlist ownership/sharing and current library grants

Regression sources: [playlists_test.go](../../core/playlists/playlists_test.go), [smart_playlist_repository_test.go](../../persistence/smart_playlist_repository_test.go), [PlaylistList.test.jsx](../../ui/src/playlist/PlaylistList.test.jsx).

## Favorites and ratings

[Source](../../persistence) · [Interface](../../ui/src/common/RatingField.jsx) · [Guide](library.md)

Personal annotations

Regression sources: [media_annotation_test.go](../../server/subsonic/media_annotation_test.go), [sql_annotations_test.go](../../persistence/sql_annotations_test.go).

## Cover art and metadata

[Source](../../core/artwork) · [Interface](../../ui/src/album/AlbumDetails.jsx) · [Guide](library.md)

Current library grants

Regression sources: [artwork_test.go](../../core/artwork/artwork_test.go), [artwork_repository_test.go](../../persistence/artwork_repository_test.go).

## Formats and transcoding

[Source](../../core/ffmpeg) · [Interface](../../ui/src/dialogs/useTranscodingOptions.jsx) · [Guide](player.md)

Signed tokens, profile selection and current library grants

Regression sources: [transcode_test.go](../../server/subsonic/transcode_test.go), [ffmpeg_test.go](../../core/ffmpeg/ffmpeg_test.go), [useTranscodingOptions.test.jsx](../../ui/src/dialogs/useTranscodingOptions.test.jsx).

## OpenSubsonic

[Source](../../server/subsonic) · [Interface](../../server/subsonic) · [Guide](player.md)

Authenticated client; current library and child policies

Regression sources: [opensubsonic_test.go](../../server/subsonic/opensubsonic_test.go), [subsonic_stream_test.go](../../server/subsonic/e2e/subsonic_stream_test.go).

## Users and explicit library grants

[Source](../../core/library.go) · [Interface](../../ui/src/user) · [Guide](users.md)

Administrator; public users start without implicit library grants

Regression sources: [library_test.go](../../core/library_test.go), [family_setup_test.go](../../server/family_setup_test.go), [userValidation.test.js](../../ui/src/user/userValidation.test.js), [SelectLibraryInput.test.jsx](../../ui/src/common/SelectLibraryInput.test.jsx).

## Administration and settings

[Source](../../server/nativeapi) · [Interface](../../ui/src/personal) · [Guide](users.md)

Role-limited administrative APIs

Regression sources: [config_test.go](../../server/nativeapi/config_test.go).

## Bunny Storage

[Source](../../core/storage/bunny) · [Interface](../../ui/src/library/ExternalSources.jsx) · [Guide](integrations/bunny.md)

Server-side credentials; library grants; add-only companion

Regression sources: [bunny_test.go](../../adapters/bunny/bunny_test.go), [bunny_test.go](../../core/storage/bunny/bunny_test.go), [test_tools.py](../../tests/bunny_ops/test_tools.py).

## S3-compatible storage

[Source](../../adapters/external/http.go) · [Interface](../../ui/src/library/ExternalSources.jsx) · [Guide](integrations/s3.md)

Administrator source configuration; user library grants

Regression sources: [protocol_test.go](../../adapters/external/protocol_test.go), [proxy_test.go](../../adapters/external/proxy_test.go), [remote_safety_test.go](../../scanner/remote_safety_test.go), [remote_test.go](../../core/storage/remote_test.go), [ExternalSources.test.jsx](../../ui/src/library/ExternalSources.test.jsx).

## SFTP

[Source](../../adapters/external/sftp.go) · [Interface](../../ui/src/library/ExternalSources.jsx) · [Guide](integrations/sftp.md)

Pinned SSH host key; library grants

Regression sources: [protocol_test.go](../../adapters/external/protocol_test.go), [proxy_test.go](../../adapters/external/proxy_test.go), [remote_safety_test.go](../../scanner/remote_safety_test.go), [remote_test.go](../../core/storage/remote_test.go), [ExternalSources.test.jsx](../../ui/src/library/ExternalSources.test.jsx).

## HTTPS WebDAV

[Source](../../adapters/external/http.go) · [Interface](../../ui/src/library/ExternalSources.jsx) · [Guide](integrations/webdav.md)

HTTPS certificate validation; library grants

Regression sources: [protocol_test.go](../../adapters/external/protocol_test.go), [proxy_test.go](../../adapters/external/proxy_test.go), [remote_safety_test.go](../../scanner/remote_safety_test.go), [remote_test.go](../../core/storage/remote_test.go), [ExternalSources.test.jsx](../../ui/src/library/ExternalSources.test.jsx).

## NAS connections

[Source](../../adapters/external/http.go) · [Interface](../../ui/src/library/ExternalSources.jsx) · [Guide](integrations/nas.md)

Connect through supported protocol; no physical NAS acceptance claimed

Regression sources: [protocol_test.go](../../adapters/external/protocol_test.go), [proxy_test.go](../../adapters/external/proxy_test.go), [remote_safety_test.go](../../scanner/remote_safety_test.go), [remote_test.go](../../core/storage/remote_test.go), [ExternalSources.test.jsx](../../ui/src/library/ExternalSources.test.jsx).

## macOS Family Sync

[Source](../../macos/navidrome_family_sync.py) · [Interface](../../macos/navidrome_family_sync.py) · [Guide](integrations/mac-sync.md)

Per-user Keychain, LaunchAgent and SQLite; remote deletion prohibited

Regression sources: [test_navidrome_family_sync.py](../../macos/test_navidrome_family_sync.py).

## Library Doctor companion

[Source](../../integrations/doctor/analyze.py) · [Interface](../../macos/navidrome_family_doctor.py) · [Guide](integrations/doctor.md)

Explicit bounded job and source budgets; no original media mutation

Regression sources: [test_analyze.py](../../tests/doctor/test_analyze.py), [test_companion.py](../../tests/doctor/test_companion.py).

## 27 per-user themes

[Source](../../ui/src/themes) · [Interface](../../ui/src/personal/SelectTheme.jsx) · [Guide](themes.md)

Personal preference; retained on restart

Regression sources: [familyThemes.test.js](../../ui/src/themes/familyThemes.test.js), [useCurrentTheme.test.jsx](../../ui/src/themes/useCurrentTheme.test.jsx), [family_preferences_test.go](../../server/nativeapi/family_preferences_test.go).

## Lyrics plugin

[Source](../../integrations/plugins/plugins.lock.json) · [Interface](../../ui/src/plugin) · [Guide](plugins/lyrics.md)

Optional explicit plugin permissions; OpenSubsonic lyrics support

Regression sources: [host_httpclient_test.go](../../plugins/host_httpclient_test.go), [family_policy_test.go](../../plugins/family_policy_test.go), [manager_loader_test.go](../../plugins/manager_loader_test.go), [lyrics_adapter_test.go](../../plugins/lyrics_adapter_test.go).

## Apple Music metadata plugin

[Source](../../integrations/plugins/plugins.lock.json) · [Interface](../../ui/src/plugin) · [Guide](plugins/apple-music.md)

Optional explicit plugin permissions and outbound endpoints

Regression sources: [host_httpclient_test.go](../../plugins/host_httpclient_test.go), [family_policy_test.go](../../plugins/family_policy_test.go), [manager_loader_test.go](../../plugins/manager_loader_test.go), [metadata_agent_test.go](../../plugins/metadata_agent_test.go).

## Cover Art Archive plugin

[Source](../../integrations/plugins/plugins.lock.json) · [Interface](../../ui/src/plugin) · [Guide](plugins/cover-art.md)

Optional explicit plugin permissions and outbound endpoints

Regression sources: [host_httpclient_test.go](../../plugins/host_httpclient_test.go), [family_policy_test.go](../../plugins/family_policy_test.go), [manager_loader_test.go](../../plugins/manager_loader_test.go), [metadata_agent_test.go](../../plugins/metadata_agent_test.go).

## Encrypted backup and full restore

[Source](../../integrations/bunny/full_backup.py) · [Interface](../../integrations/bunny/full_backup.py) · [Guide](backups.md)

Operator-only AES-GCM; stopped complete volume; separate secrets

Regression sources: [test_full_backup.py](../../tests/bunny_ops/test_full_backup.py), [test_family_backup.py](../../tests/bunny_ops/test_family_backup.py), [selfhost_acceptance.py](../../tests/selfhost_acceptance.py).

## Protected first-run setup

[Source](../../server/family_setup.go) · [Interface](../../ui/src/layout/Login.jsx) · [Guide](quickstart.md)

One-time setup token and serialized first administrator creation

Regression sources: [family_setup_test.go](../../server/family_setup_test.go), [test_selfhost.py](../../tests/test_selfhost.py), [selfhost_acceptance.py](../../tests/selfhost_acceptance.py).

