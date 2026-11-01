# Listen, queue and playlists

Open an album and press **Play**. The player at the bottom provides play/pause, previous/next, progress, volume and queue. Click the progress bar to seek; the source and format must support the requested range or transcoding. Open the queue to reorder or remove queued tracks. Queue removal does not delete source files.

Use a song or album action menu to add music to the queue or a playlist. Create a playlist from **Playlists**, give it a name and add tracks. Edit its name, order and sharing using its menu. Private playlists stay personal; shared playlists still obey the listener's library permissions. Deleting a playlist deletes its list, not audio files. Smart playlists use server-side rules rather than a hand-maintained list; their `.nsp` files and rule format follow the upstream [smart playlist guide](https://www.navidrome.org/docs/usage/smartplaylists/).

Direct playback depends on the browser's codecs. MP3 is widely supported; FLAC support varies by browser/device. Other indexed formats may require server FFmpeg transcoding. In personal/player settings choose a supported transcoding profile and bitrate when needed. Lossless source metadata does not prove that the active output is lossless: a chosen transcode changes the stream. See [Quality Passport](features/passport.md).

OpenSubsonic clients connect to your own HTTPS server URL with that person's credentials. Do not use the administrator account for ordinary mobile listening. Client features differ, including synchronized lyrics, offline downloads and codec support. No physical phone acceptance is implied by browser tests.

If sound does not start, click Play yourself to satisfy browser autoplay rules, check mute/output device, source availability and network. For seek failures check Range support and the chosen transcode. Never paste a full authenticated stream URL into a public issue.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Unofficial Navidrome fork; GPLv3.
