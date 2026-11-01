# macOS Family Sync

[English](mac-sync.md) · [Русский](../../ru/integrations/mac-sync.md)

Use this optional Mac companion when your server reads music from your own private Bunny Storage zone. It watches `~/Music/Navidrome` and uploads new stable files **Mac → Bunny only**. It never deletes remote music or silently overwrites a name conflict. A basic local Docker installation needs no Bunny sync agent: its read-only music folder is already local.

## Install once

You need macOS, Python 3.10+, your existing Bunny Storage write credential and your own Navidrome administrator password. Do not create a registry PAT for this uploader; it does not use a Docker registry.

1. From the source checkout, run the installer with your own HTTPS server, zone and administrator name:

   ```sh
   python3 macos/navidrome_family_sync.py install \
     --music-dir "$HOME/Music/Navidrome" \
     --server https://music.example.com --zone YOUR_ZONE --owner YOUR_ADMIN
   python3 macos/navidrome_family_sync.py setup-keychain
   python3 macos/navidrome_family_sync.py status
   ```

2. Replace all example values. Enter the existing credentials only at the secure interactive Keychain prompts. The agent does not need the Bunny account API key. Secrets stay in macOS Keychain, not the plist, config, arguments or logs.
3. Check `installed: true`, `paused: false` and `interval_seconds: 300`. The per-user LaunchAgent has RunAtLoad and runs in your logged-in GUI session. Allow macOS notifications if prompted.
4. Copy an album into the folder. A file must remain unchanged across two checks at least 300 seconds apart. Normal discovery therefore takes about 5–10 minutes while the Mac is awake; large uploads take longer. After verified uploads, the agent requests an authenticated server scan.
5. Open the album in your server and play it. An unchanged subsequent poll skips local rehashing and remote GET/PUT requests. Removing a local copy leaves the Bunny object intact.

## Daily controls

```sh
python3 macos/navidrome_family_sync.py sync-now
python3 macos/navidrome_family_sync.py pause
python3 macos/navidrome_family_sync.py resume
python3 macos/navidrome_family_sync.py logs --lines 40
python3 macos/navidrome_family_sync.py uninstall
```

The manifest is local SQLite in the user's Application Support directory. Uninstall removes the LaunchAgent, retaining music, manifest and Keychain items. A short-lived job being “not running” with last exit code 0 is normal between polls.

If the Internet or Keychain is unavailable, the agent records bounded failure and retries later. Name conflicts are held rather than repeatedly downloading the remote file. A failed server scan remains pending. After sleep/login, missed intervals are coalesced by launchd; no upload happens while the Mac is powered off. Rename can create a second remote path, so deliberate remote cleanup is a separate operator action, never automatic sync behavior.

## Verified scope

Functional acceptance was performed on a real Mac: Keychain, scheduled MP3/FLAC uploads, SHA-256, Range, scan, unchanged polls, conflict refusal, local-deletion retention, reboot and later sleep/wake continuation. Visible notification-banner observation remains **NOT_RUN**; a successful notification command is not proof that macOS displayed a banner. No full MACOS_ACCEPTANCE_PASS label is claimed for that unobserved presentation detail. Your own Mac/OS permission settings still need verification.
