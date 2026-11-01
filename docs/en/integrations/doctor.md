# Library Doctor Companion on Mac

The server performs bounded metadata checks. Deep SHA-256, decoded PCM and Chromaprint analysis runs only when you explicitly start the Mac companion. It is not an always-running cloud worker and never repairs, deletes or retags original music.

1. Install Docker Desktop and build the local tool from the same release: `docker build --platform linux/amd64 -f Dockerfile.family-doctor -t navidrome-family-doctor:local .` Keep the image local; the companion refuses to pull a missing image automatically.
2. Open **Family Home → More → Library Doctor** as administrator. Select libraries, limits and requested checks. Read the estimated work and create a bounded deep job. Copy its job ID.
3. The Mac companion uses the existing administrator password in macOS Keychain, the same service as [Family Sync](mac-sync.md). Do not put a password in arguments or a JSON file. Set up the existing credential through Sync's secure interactive installer if necessary; no new token is required.
4. Run from the release directory:

```sh
python3 macos/navidrome_family_doctor.py --server https://music.example.invalid --owner admin --job YOUR_JOB_ID
```

Replace the example endpoint, username and job ID with your own. If Sync is already configured, its server/username supply the defaults. The default tool image is `navidrome-family-doctor:local`.

5. Watch the server job page. Each local analysis container is network-isolated, read-only, capability-dropped and limited to 2 CPUs, 512 MB, 64 processes and a small temporary filesystem. Remote audio is fetched by the authorized companion and handled privately; only bounded analysis results return to the server.
6. Use Ctrl-C to request pause; progress remains in server SQLite. Resume with the same command plus `--resume`; use `--retry` only after reviewing a failed item. Do not start two companions for the same work.

A matching filename/title is not proof of an acoustical duplicate. A high sample rate is not proof of a true high-resolution source. Inspect confidence and unknown/incomplete results in [Quality Passport](../features/passport.md). Missing original, unavailable provider, unsupported decoder or deadline produces an incomplete result rather than a fabricated verdict. For failures check Docker Desktop, local image labels/version, existing Keychain access and job state. Never upload personal audio, credentials or deep-analysis caches to an issue.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Unofficial Navidrome fork; GPLv3.
