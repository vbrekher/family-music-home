# API and architecture

The Go application serves the React WebUI and authenticated Native API under `/api`. OpenSubsonic endpoints are under `/rest`; supported client compatibility is inherited from Navidrome. Authentication uses server-issued sessions/tokens; do not put them in documentation URLs.

| Layer | Source | Responsibility |
|---|---|---|
| Server and first run | `server/`, `integrations/selfhost/` | Routes, login, single administrator bootstrap and setup-token checks |
| Native API | `server/nativeapi/` | User/library operations and authorized family resources |
| Family domain | `core/family/` | Shelves, letters, capsules, requests, Doctor, versions, Kids, Legacy and policies |
| Persistence | `persistence/`, `db/migrations/` | SQLite repositories and versioned migrations |
| Storage | `core/storage/`, `adapters/`, `integrations/external_sources/` | Local/Bunny/S3/SFTP/WebDAV metadata and streaming |
| WebUI | `ui/src/` | Inherited music UI, family screens, forms, themes and player |
| Plugins | `plugins/`, `integrations/plugins/` | Capability-limited WASM plugin host and pinned integration manifests |
| Mac tools | `macos/` | Keychain-backed add-only sync and bounded local Doctor companion |
| Backup | `integrations/bunny/` | Online SQLite backup plus separate complete stopped-volume backup |

Family resources are not a second account system. They use existing users, library grants and protected-child policies. A reference to an album does not authorize a stream. Recipient checks and media ownership are enforced server-side. Structured writes use revision checks and explicit action endpoints to avoid silently overwriting concurrent changes.

Remote source configuration is administrator-only and refers to secret environment names. Read-only protocols do not automatically copy an entire remote collection locally. Temporary offline state preserves catalog identity; original source deletion is not a supported automatic sync action.

SQLite needs one writer process per persistent volume. Plugin KV databases and private family media are separate files; a complete recovery snapshot includes them. Rebuildable caches are excluded by the complete backup helper.

This document is an architectural map, not a promise that private implementation APIs remain stable. Consult source route definitions and tests for exact request schemas; expose integrations through OpenSubsonic where appropriate.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Unofficial Navidrome fork; GPLv3.
