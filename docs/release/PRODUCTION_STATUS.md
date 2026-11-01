# Personal production status

The public source release and the maintainer’s personal deployment are separate acceptance gates.

The existing personal Bunny application, image, storage zone, accounts and 5 GB persistent volume have been preserved. No production rollout was performed for this release.

An image-only update requires a fresh, consistent, encrypted **complete** production data-volume backup, a successful restore with both existing adult accounts, and verification of the existing private registry image digest. A database-only online backup does not include plugin packages and plugin key/value databases and does not satisfy that requirement.

The operator-accessible official Magic Containers API and CLI were checked, including CLI 0.21.1. They expose application deployment and volume listing/removal, but no supported full-volume export, filesystem session or snapshot-download operation was available through that access. No undocumented API or temporary command/sidecar workaround was used.

A stopped local copy was backed up and restored successfully, including all three plugin packages and two plugin stores, with both adult logins verified. This demonstrates the backup implementation; it is **not** a fresh production backup. The remaining production blocker is provider-supported complete-volume backup/read access. The public self-host product can be built and installed independently of that personal deployment.
