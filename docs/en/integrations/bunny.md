# Bunny Storage music

Bunny is optional. A local installation needs only your own music folder. Use this integration when you already operate a private Bunny Storage zone and want the server to read music without copying the entire collection into `/data`.

1. Keep your zone private and use its correct HTTPS regional origin. Store `BUNNY_STORAGE_ZONE`, `BUNNY_STORAGE_ACCESS_KEY` and optional `BUNNY_STORAGE_ORIGIN` in the server's protected environment/secret store. Use a read credential for playback; the Mac uploader separately needs write access. Never put the key in a library URI or browser screenshot.
2. As administrator open **Libraries** → **Create**, choose a name and the path `bunny://YOUR_ZONE/music` (replace both the zone and prefix with your own). The URI must match the configured zone and cannot contain a username, password, query or fragment.
3. Save and scan. Inspect albums and play one MP3 and FLAC. Test seeking after playback starts. Streaming is authorized by your server; a shared family reference does not publish the zone credential.
4. Grant this library to the intended [members](../users.md), then test with that account. Do not grant administrator rights for listening.
5. For a temporary outage, restore the connection and rescan. Keep the existing catalog. Remove only a connection/library configuration when intentionally retiring it; never delete original objects as a reconnect workaround.

Use [macOS Family Sync](mac-sync.md) to upload from your Mac when needed. Scanning and the read-only storage adapter do not automatically upload or delete music. [Encrypted backups](../backups.md) use a separate explicit write configuration (`BUNNY_STORAGE_PASSWORD` and backup key); music playback's read key is not a backup-encryption key.

See the official [Bunny Storage documentation](https://docs.bunny.net/) for your zone's origin and credential roles. Provider availability and your network still determine playback; a local protocol test is distinct from a real hosted-origin acceptance.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Unofficial Navidrome fork; GPLv3.
