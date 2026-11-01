# Family Music Home 1.0.0

An unofficial free GPLv3 Navidrome fork with complete server/UI/integration source, all eleven family modules, 27 themes, read-only external Bunny/S3/SFTP/HTTPS WebDAV libraries, three optional metadata plugins, protected self-host initialization and paired English/Russian guides. Existing Navidrome browsing, player, playlists, favorites, ratings, transcoding and OpenSubsonic remain available.

[Quick start](quickstart.md) · [Gallery](gallery.md) · [Feature matrix](feature-matrix.md) · [Developer guide](developer.md).

The release was built and tested locally on macOS with Docker Desktop for linux/amd64. See the [acceptance report](../release/ACCEPTANCE.md), [security review](../release/SECURITY_REVIEW.md) and [UI audit](../design/UI_AUDIT.md) for observed evidence and explicit limits. Development dependencies retain 22 moderate advisories; runtime npm, Python and Alpine scans are clean. Go's module inventory includes an unused OpenPGP advisory, with zero imported-package/reachable findings.

Physical phones/tablets/NAS, native browser 200% zoom and the visible macOS notification banner are NOT_RUN. New family-screen labels are currently English; inherited Navidrome translations and the English/Russian guides are retained. No public demonstration server or runtime image is published.

The maintainer’s personal Bunny server is preserved, without a production rollout. Provider-supported complete-volume backup access is still required before an image-only update; local backup rehearsal does not satisfy the fresh production backup gate. [Production status](../release/PRODUCTION_STATUS.md).

Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Upstream copyright and dependency notices are retained.
