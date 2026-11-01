# Security review — 1.0.0

Local source and Docker review, 2026-11-01. This is an engineering verification, not an independent penetration-test certification.

* Public export uses an explicit path/SHA-256 allowlist and a fresh history. Personal infrastructure, database files, credentials, private reports, old Git history, workflows and commercial test media are excluded. Upstream test fixtures are fetched separately by an explicit, checksum-pinned development command.
* Gitleaks reviewed source scan: zero findings. Inventory hashes and three documented synthetic test values receive narrowly scoped exceptions; credential rules remain enabled.
* Runtime npm dependency audit: zero vulnerabilities. Full development audit: 22 moderate advisories in development tooling, zero high/critical. Legacy glob/lint/workbox chains remain under review; there is no Node runtime service. Do not interpret the runtime result as a zero-advisory development tree.
* Python runtime audit: zero findings for cryptography 50.0.2, cffi 2.1.1 and pycparser 3.12. Pip is removed from the final runtime image.
* Alpine security updates are installed at build time. Trivy 0.76.1 rescan: zero Alpine and Python findings. Its Go binary inventory reports GO-2026-5932 for the required x/crypto module; the unmaintained openpgp package is not imported. Go vulnerability analysis reports zero imported-package and reachable vulnerabilities. go-chi is updated to 5.4.0.
* First-administrator setup requires a random setup token and a password of at least twelve characters. Concurrent initialization creates exactly one administrator. Subsequent accounts receive no implicit library grants in public self-host mode. Revocation is checked on streams and family references.
* Signed streaming parameters are redacted as a whole in request logs. Child playback retains normal library authorization; forbidden methods and revoked grants fail closed.
* External-source connectors enforce transport, redirect, address and credential boundaries. SFTP pins its host key; storage mounts and production music remain read-only. Real physical NAS acceptance is not claimed.
* Backup restoration authenticates before extracting, rejects path traversal and symlinks, checks SQLite and includes plugin packages and plugin state. Consistency requires stopped writers; a database-only export is not a full-volume backup.

The CycloneDX npm runtime inventory and Go/npm license inventories are in this directory. Dependencies are not made GPL merely by appearing in an inventory; their original notices and terms are retained. No public runtime image or binary is distributed with this source release.

Report suspected vulnerabilities using [SECURITY.md](../../SECURITY.md). No production secrets or private server identifiers belong in public issues.
