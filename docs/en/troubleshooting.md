# Troubleshooting

| Symptom | What to check |
|---|---|
| Browser cannot connect | Docker Desktop is running; `docker compose -f compose.family.yml ps`; correct localhost port; no other process occupies it. |
| Startup refused / permission error | Data volume must be writable by the container user. A manually specified UID needs an operator-prepared directory/volume with matching ownership. Do not make it world-writable. |
| Setup form refuses creation | Read the token from this instance, use a unique permitted username and a password of at least 12 characters. If accounts already exist, use normal login. |
| No music | Host music path exists, read-only mount points to it, scan completed, tags are valid and current user has a library grant. |
| Remote music unavailable | Test Connection, DNS/network, provider status, configured secret, TLS/SSH verification. Keep the catalog while reconnecting. |
| Playback fails | Click Play manually, check browser codec and output/mute; try a supported transcode; inspect server logs privately. |
| Seek fails | Provider must support Range or the active transcoding path; test another supported format. |
| Family item or attachment missing | Check author/recipient, archive/filter state, music grants and backup completeness. Do not grant administrator rights as a workaround. |
| Plugin has no result | Enable approved permissions, check its provider, supported metadata and logs; absence of lyrics/art is not proof of a broken library. |
| Mac upload not running | See [Mac diagnostics](integrations/mac-sync.md): agent enabled, Keychain access, network and next 300-second poll. |

Before an issue, record the release, browser, operating system and a small synthetic reproduction. Review logs for tokens, addresses, filenames and people. Never attach `/data` or a full backup. Back up before changing deployment or database settings.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Unofficial Navidrome fork; GPLv3.
