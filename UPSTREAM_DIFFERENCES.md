# Differences from Navidrome

Family Music Home is an unofficial GPLv3 fork, based on Navidrome v0.64.2. The original project supplies the catalog, tag reader, scanner, player, queues, playlists/smart playlists, favorites, ratings, transcoding, OpenSubsonic/Jellyfin foundation, user/library management, plugin runtime and theme infrastructure.

This fork adds eleven family workflows: Home Dashboard, Shelves, Time Capsule, Music Letters, Music Requests, Forgotten Albums, Quality Passport, Library Doctor, Version Collector, Kids Music World and Legacy Export/Verify. It adds private attachments and encrypted recovery, read-only Bunny/S3/SFTP/HTTPS WebDAV storage adapters and management, per-user theme persistence, macOS Family Sync and the bounded local diagnostic companion.

Lyrics, Apple Music metadata and Cover Art Archive are third-party plugins, not newly authored provider implementations. Their manifests are restricted and checksum pinned. No Album Experience or Listen Together room feature is included. Ordinary album playback remains inherited Navidrome behavior.

The public Docker installation starts with no accounts and needs no Bunny service. Existing protected deployments retain their separate guarded bootstrap, persistent data and identities. Neither the upstream project nor the fork provides hosting; users pay their own optional infrastructure costs.

Created by [@vbrekher](https://github.com/vbrekher). Thanks to my beloved wife [@krisbrekher](https://github.com/krisbrekher) for the ideas and inspiration. ❤️
