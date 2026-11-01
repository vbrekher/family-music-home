# Update without losing your library

Updates can migrate SQLite. An old image alone is not a rollback plan.

1. Record the current source revision, immutable image ID/digest and deployment settings. Keep the matching image locally.
2. Make a [complete encrypted backup](backups.md), restore it separately and verify administrator/member logins, music access and family attachments. Include plugin packages and KV databases.
3. Obtain the new reviewed source release. Build locally with `docker compose -f compose.family.yml build`; do not remove the data volume or music directory.
4. Start with `docker compose -f compose.family.yml up -d`. Check `/ping`, sign in, play MP3 and FLAC, test seeking and inspect your shelves, letters, memories and plugin settings.
5. Verify a new backup after the update. Keep the pre-update image and matching snapshot until acceptance is complete.

For rollback stop the application and restore the matching old complete snapshot into a separate volume; start it with the old exact image and the same required secrets. Test before switching traffic. Never run two writers against the same SQLite volume. Do not use `docker compose down -v` as an update step.

Remote hosting needs an official way to export/restore the complete data volume. If that access is unavailable, stop the rollout and keep the existing server running. Do not invent an API or substitute a SQLite-only snapshot for a full recovery plan.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Unofficial Navidrome fork; GPLv3.
