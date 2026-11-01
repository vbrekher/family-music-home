# Матрица функций

Инвентаризация охватывает исходный сервер, хранилища, вспомогательные интеграции и одиннадцать семейных модулей. Локальные тесты, протокольная приёмка и production — разные области проверки. Unit-тест не подтверждает физический NAS, телефон или обновление личного сервера.

[Матрица JSON](../release/feature-matrix.json) · [Семейные модули JSON](../release/family-feature-matrix.json)

## Family Home Dashboard

[Source](../../server/nativeapi/family_home.go) · [Interface](../../ui/src/family/FamilyHome.jsx) · [Guide](features/home.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Family Shelves

[Source](../../core/family/store.go) · [Interface](../../ui/src/family/Shelves.jsx) · [Guide](features/shelves.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Family Music Time Capsule

[Source](../../core/family/store.go) · [Interface](../../ui/src/family/Journal.jsx) · [Guide](features/capsules.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Music Letters

[Source](../../core/family/letters.go) · [Interface](../../ui/src/family/Journal.jsx) · [Guide](features/letters.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Family Music Requests

[Source](../../core/family/requests.go) · [Interface](../../ui/src/family/Requests.jsx) · [Guide](features/requests.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Forgotten Albums

[Source](../../server/nativeapi/family_home.go) · [Interface](../../ui/src/family/FamilyHome.jsx) · [Guide](features/forgotten.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Music Quality Passport

[Source](../../server/nativeapi/family_home.go) · [Interface](../../ui/src/family/Passport.jsx) · [Guide](features/passport.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Library Doctor

[Source](../../core/family/doctor_jobs.go) · [Interface](../../ui/src/family/Doctor.jsx) · [Guide](features/doctor.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Version Collector

[Source](../../core/family/versions.go) · [Interface](../../ui/src/family/Versions.jsx) · [Guide](features/versions.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Kids Music World

[Source](../../core/family/children.go) · [Interface](../../ui/src/family/Children.jsx) · [Guide](features/kids.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Family Legacy Export

[Source](../../core/family/legacy.go) · [Interface](../../ui/src/family/Legacy.jsx) · [Guide](features/legacy.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [family](../../core/family/), [nativeapi](../../server/nativeapi/), [family](../../ui/src/family/).

## Music catalog, artists and albums

[Source](../../persistence) · [Interface](../../ui/src/album) · [Guide](library.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [album_repository_test.go](../../persistence/album_repository_test.go), [artist_repository_test.go](../../persistence/artist_repository_test.go), [AlbumDetails.test.jsx](../../ui/src/album/AlbumDetails.test.jsx).

## Genres, search, filters and sorting

[Source](../../server/nativeapi) · [Interface](../../ui/src/artist) · [Guide](library.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [genre_repository_test.go](../../persistence/genre_repository_test.go), [sql_search_fts_test.go](../../persistence/sql_search_fts_test.go), [searching_test.go](../../server/subsonic/searching_test.go).

## Web player and queue

[Source](../../server/subsonic) · [Interface](../../ui/src/audioplayer) · [Guide](player.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [media_retrieval_test.go](../../server/subsonic/media_retrieval_test.go), [playqueue_repository_test.go](../../persistence/playqueue_repository_test.go), [AudioTitle.test.jsx](../../ui/src/audioplayer/AudioTitle.test.jsx).

## Playlists and smart playlists

[Source](../../core/playlists) · [Interface](../../ui/src/playlist) · [Guide](library.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [playlists_test.go](../../core/playlists/playlists_test.go), [smart_playlist_repository_test.go](../../persistence/smart_playlist_repository_test.go), [PlaylistList.test.jsx](../../ui/src/playlist/PlaylistList.test.jsx).

## Favorites and ratings

[Source](../../persistence) · [Interface](../../ui/src/common/RatingField.jsx) · [Guide](library.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [media_annotation_test.go](../../server/subsonic/media_annotation_test.go), [sql_annotations_test.go](../../persistence/sql_annotations_test.go).

## Cover art and metadata

[Source](../../core/artwork) · [Interface](../../ui/src/album/AlbumDetails.jsx) · [Guide](library.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [artwork_test.go](../../core/artwork/artwork_test.go), [artwork_repository_test.go](../../persistence/artwork_repository_test.go).

## Formats and transcoding

[Source](../../core/ffmpeg) · [Interface](../../ui/src/dialogs/useTranscodingOptions.jsx) · [Guide](player.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [transcode_test.go](../../server/subsonic/transcode_test.go), [ffmpeg_test.go](../../core/ffmpeg/ffmpeg_test.go), [useTranscodingOptions.test.jsx](../../ui/src/dialogs/useTranscodingOptions.test.jsx).

## OpenSubsonic

[Source](../../server/subsonic) · [Interface](../../server/subsonic) · [Guide](player.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [opensubsonic_test.go](../../server/subsonic/opensubsonic_test.go), [subsonic_stream_test.go](../../server/subsonic/e2e/subsonic_stream_test.go).

## Users and explicit library grants

[Source](../../core/library.go) · [Interface](../../ui/src/user) · [Guide](users.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [library_test.go](../../core/library_test.go), [family_setup_test.go](../../server/family_setup_test.go), [userValidation.test.js](../../ui/src/user/userValidation.test.js), [SelectLibraryInput.test.jsx](../../ui/src/common/SelectLibraryInput.test.jsx).

## Administration and settings

[Source](../../server/nativeapi) · [Interface](../../ui/src/personal) · [Guide](users.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [config_test.go](../../server/nativeapi/config_test.go).

## Bunny Storage

[Source](../../core/storage/bunny) · [Interface](../../ui/src/library/ExternalSources.jsx) · [Guide](integrations/bunny.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [bunny_test.go](../../adapters/bunny/bunny_test.go), [bunny_test.go](../../core/storage/bunny/bunny_test.go), [test_tools.py](../../tests/bunny_ops/test_tools.py).

## S3-compatible storage

[Source](../../adapters/external/http.go) · [Interface](../../ui/src/library/ExternalSources.jsx) · [Guide](integrations/s3.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [protocol_test.go](../../adapters/external/protocol_test.go), [proxy_test.go](../../adapters/external/proxy_test.go), [remote_safety_test.go](../../scanner/remote_safety_test.go), [remote_test.go](../../core/storage/remote_test.go), [ExternalSources.test.jsx](../../ui/src/library/ExternalSources.test.jsx).

## SFTP

[Source](../../adapters/external/sftp.go) · [Interface](../../ui/src/library/ExternalSources.jsx) · [Guide](integrations/sftp.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [protocol_test.go](../../adapters/external/protocol_test.go), [proxy_test.go](../../adapters/external/proxy_test.go), [remote_safety_test.go](../../scanner/remote_safety_test.go), [remote_test.go](../../core/storage/remote_test.go), [ExternalSources.test.jsx](../../ui/src/library/ExternalSources.test.jsx).

## HTTPS WebDAV

[Source](../../adapters/external/http.go) · [Interface](../../ui/src/library/ExternalSources.jsx) · [Guide](integrations/webdav.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [protocol_test.go](../../adapters/external/protocol_test.go), [proxy_test.go](../../adapters/external/proxy_test.go), [remote_safety_test.go](../../scanner/remote_safety_test.go), [remote_test.go](../../core/storage/remote_test.go), [ExternalSources.test.jsx](../../ui/src/library/ExternalSources.test.jsx).

## NAS connections

[Source](../../adapters/external/http.go) · [Interface](../../ui/src/library/ExternalSources.jsx) · [Guide](integrations/nas.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [protocol_test.go](../../adapters/external/protocol_test.go), [proxy_test.go](../../adapters/external/proxy_test.go), [remote_safety_test.go](../../scanner/remote_safety_test.go), [remote_test.go](../../core/storage/remote_test.go), [ExternalSources.test.jsx](../../ui/src/library/ExternalSources.test.jsx).

## macOS Family Sync

[Source](../../macos/navidrome_family_sync.py) · [Interface](../../macos/navidrome_family_sync.py) · [Guide](integrations/mac-sync.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [test_navidrome_family_sync.py](../../macos/test_navidrome_family_sync.py).

## Library Doctor companion

[Source](../../integrations/doctor/analyze.py) · [Interface](../../macos/navidrome_family_doctor.py) · [Guide](integrations/doctor.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [test_analyze.py](../../tests/doctor/test_analyze.py), [test_companion.py](../../tests/doctor/test_companion.py).

## 27 per-user themes

[Source](../../ui/src/themes) · [Interface](../../ui/src/personal/SelectTheme.jsx) · [Guide](themes.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [familyThemes.test.js](../../ui/src/themes/familyThemes.test.js), [useCurrentTheme.test.jsx](../../ui/src/themes/useCurrentTheme.test.jsx), [family_preferences_test.go](../../server/nativeapi/family_preferences_test.go).

## Lyrics plugin

[Source](../../integrations/plugins/plugins.lock.json) · [Interface](../../ui/src/plugin) · [Guide](plugins/lyrics.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [host_httpclient_test.go](../../plugins/host_httpclient_test.go), [family_policy_test.go](../../plugins/family_policy_test.go), [manager_loader_test.go](../../plugins/manager_loader_test.go), [lyrics_adapter_test.go](../../plugins/lyrics_adapter_test.go).

## Apple Music metadata plugin

[Source](../../integrations/plugins/plugins.lock.json) · [Interface](../../ui/src/plugin) · [Guide](plugins/apple-music.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [host_httpclient_test.go](../../plugins/host_httpclient_test.go), [family_policy_test.go](../../plugins/family_policy_test.go), [manager_loader_test.go](../../plugins/manager_loader_test.go), [metadata_agent_test.go](../../plugins/metadata_agent_test.go).

## Cover Art Archive plugin

[Source](../../integrations/plugins/plugins.lock.json) · [Interface](../../ui/src/plugin) · [Guide](plugins/cover-art.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [host_httpclient_test.go](../../plugins/host_httpclient_test.go), [family_policy_test.go](../../plugins/family_policy_test.go), [manager_loader_test.go](../../plugins/manager_loader_test.go), [metadata_agent_test.go](../../plugins/metadata_agent_test.go).

## Encrypted backup and full restore

[Source](../../integrations/bunny/full_backup.py) · [Interface](../../integrations/bunny/full_backup.py) · [Guide](backups.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [test_full_backup.py](../../tests/bunny_ops/test_full_backup.py), [test_family_backup.py](../../tests/bunny_ops/test_family_backup.py), [selfhost_acceptance.py](../../tests/selfhost_acceptance.py).

## Protected first-run setup

[Source](../../server/family_setup.go) · [Interface](../../ui/src/layout/Login.jsx) · [Guide](quickstart.md)

Права, настройки и порядок работы описаны в связанном русском руководстве.

Исходники проверок: [family_setup_test.go](../../server/family_setup_test.go), [test_selfhost.py](../../tests/test_selfhost.py), [selfhost_acceptance.py](../../tests/selfhost_acceptance.py).

