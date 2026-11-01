# Release acceptance — 1.0.0

Verification date: 2026-11-01. Builds and tests ran on the maintainer’s Mac with local Docker Desktop, targeting linux/amd64. No hosted builds, GitHub Actions or public runtime image are used.

| Area | Observed result | Scope |
|---|---|---|
| Go regression | 85 test-bearing packages passed; no failed packages | Sanitized source plus explicitly fetched, pinned public upstream fixtures; UID 502 |
| UI regression | 802 tests in 98 files passed, including final detail-navigation and Home-order regressions | Happy DOM 21, two workers, 60-second timeout |
| Family workflows | 191 guarded HTTP checks passed | Four local acceptance suites; real database, sessions, permission changes and preservation assertions |
| Child signed streams | Four checks passed | Real local MP3/FLAC Range, revoked grant, tampered signed token, complete signed-query redaction |
| Operations | 33 public-tree tests passed | Runtime, bootstrap, online/full backups, upload safety; private-only lifecycle tests excluded |
| Clean first run and restore | Nine checks passed | One concurrent first administrator, member without implicit grants, six original demo albums, both formats, revoke-all, restart, full encrypted restored clone with both logins |
| Complete private-copy restore | Passed | Stopped source, all three plugin packages and two plugin stores, both existing adult logins; not a production backup |
| Metadata plugins | All three enabled without last error | Lyrics returned 28 synchronized lines; Apple biography/image and three similar artists; Cover Art Archive returned JPEG. Provider cache may satisfy artwork; fresh outbound request is not claimed |
| External sources | Protocol integration tests passed | TLS S3 listing/pagination/signing/Range, HTTPS WebDAV, pinned-host SFTP and address/proxy boundaries; not a physical NAS |
| macOS sync | 20 tests passed on macOS; installed agent preserved | Keychain, 300 seconds, RunAtLoad, add-only behavior, unchanged-file skip; prior sleep/reboot evidence retained. Visible notification banner NOT_RUN; no MACOS_ACCEPTANCE_PASS claim |
| Doctor companion | Eight Docker and three macOS tests passed | Opt-in bounded local investigations; integrity/provenance unknowns remain unknown |
| Themes and responsiveness | 27 bundled themes switched; eight CSS widths inspected | Desktop browser emulation, not physical devices; see UI audit |
| Privacy and licenses | Reviewed explicit source export; zero reviewed-source Gitleaks findings; runtime layer deny scan passed | Final release tree/history/assets receive a fresh publication check; GPLv3 with retained upstream and dependency notices |

## Performance measurements

Two-CPU, 1 GiB local container, five samples after two warmups. Each sample made nine dashboard-data HTTP requests while other local regression builds were active. At 100 / 1,000 / 10,000 visible recordings, median elapsed times were 152.487 / 126.824 / 124.648 ms; observed fifth-sample percentile/max values were 182.256 / 136.451 / 182.527 ms. SQLite integrity was `ok`. This small local HTTP measurement excludes browser painting/artwork and does not establish production latency or a formal statistical p95.

## Explicit limits

The personal Bunny release candidate is **not deployed**: see [production status](PRODUCTION_STATUS.md). Native browser 200% zoom, physical phones/tablets/NAS and the visible macOS notification banner are **NOT_RUN**. New family labels are currently English; inherited Navidrome translations and the paired EN/RU guides remain available. No online public demo is published.

[UI audit](../design/UI_AUDIT.md) · [Feature matrix](feature-matrix.json) · [Security review](SECURITY_REVIEW.md) · [Gallery](../en/gallery.md).

Created by [@vbrekher](https://github.com/vbrekher), with heartfelt thanks to my wife [@krisbrekher](https://github.com/krisbrekher) for her ideas. Unofficial Navidrome fork, GPLv3.
