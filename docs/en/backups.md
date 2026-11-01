# Back up and restore

[English](backups.md) · [Русский](../ru/backups.md)

A personal Legacy ZIP is not a server backup and is not encrypted. Keep a separate encrypted server copy, its decryption key, your original music and your server secrets. Losing the password-encryption/server keys can prevent a restored installation from opening existing credentials.

## Choose the right copy

| Copy | Contains | Does not contain |
|---|---|---|
| Legacy ZIP | Your permitted JSON/Markdown/M3U history and selected attachments | Original music, other adults’ private records, encryption, full server restore |
| Online NDBACK01/02 | A WAL-safe SQLite snapshot; NDBACK02 also includes referenced private family media | Separate plugin KV databases, installed plugin packages, original music, server environment secrets |
| Stopped FMHFULL1 | Complete application data volume, including plugin packages/KV databases and private attachments; rebuildable root cache excluded | Music mounted outside /data, server environment secrets, external storage |

All encrypted formats use authenticated AES-256-GCM. Keep the 32-byte key (64 hexadecimal characters) in your password manager or OS Keychain, separately from the archive. Never put it in a Git issue, screenshot or command argument. Store at least one encrypted copy away from the application volume.

## Complete local Docker backup

1. Record the exact image and container before stopping it. Create a protected output folder:

   ```sh
   FMH_CONTAINER=$(docker compose -f compose.family.yml ps -q music)
   FMH_IMAGE=$(docker inspect "$FMH_CONTAINER" --format '{{.Image}}')
   mkdir -p backups
   chmod 700 backups
   docker compose -f compose.family.yml stop music
   ```

2. With the application stopped, run the full-volume helper. It asks for your backup key without echoing it. Choose a new archive name each time:

   ```sh
   docker run --rm -it --volumes-from "$FMH_CONTAINER":ro \
     -v "$PWD/backups:/backup" --entrypoint python "$FMH_IMAGE" \
     /app/bunny/full_backup.py backup /data /backup/family-full.enc
   docker compose -f compose.family.yml start music
   ```

3. Check the success message and keep the image ID alongside your private backup record. If the helper fails, restart the original application and investigate; do not delete its volume. The helper refuses existing archive destinations, source symlinks, changing files and data beyond its bounds.

For unattended operation, `--key-stdin` reads the key from standard input so an existing secret manager can supply it without placing it in arguments. This does not make it safe to run a complete volume copy against a live server. Stop every process that writes that volume first.

## Test restoration before relying on a backup

Restore into a new directory or disposable volume, never over the running database:

```sh
docker run --rm -it -v "$PWD/backups:/backup" \
  --entrypoint python "$FMH_IMAGE" /app/bunny/full_backup.py \
  restore /backup/family-full.enc /backup/restored-check
```

Use the original matching image and retained server secrets to start an isolated test instance with this restored data. Verify administrator/member logins, library grants, playlists/favorites, letters/memories/attachments, installed plugins and their saved state. The helper authenticates the whole archive before processing members, rejects traversal/links/duplicate paths, and checks the application and plugin SQLite databases. A successful decrypt alone does not prove that the application works.

The restored directory contains private plaintext data. Protect it and remove only your disposable test copy after verification. Keep original music and the encrypted archive untouched. Record real observations; do not mark a restore PASS without signing in to the restored application.

## Online Bunny backups and rollback

The optional runtime backup worker is enabled for a public install with `FMH_BUNNY_BACKUPS=true` and valid server-side Bunny Storage settings plus `BUNNY_BACKUP_KEY_HEX`. It creates and verifies encrypted database/media snapshots in approved backup prefixes. It does not delete music. Its retention policy can prune only exact backup names after an independent restore check.

For a full cloud-volume copy, use an officially supported way to stop/quiesce and read your platform’s volume. Do not substitute a live WAL-file copy or invent a filesystem/exec API. If your platform exposes no supported volume access, record that limit and do not claim a complete plugin/data backup. Before an image migration, require a fresh verified snapshot covering the actual application state.

Rollback means restoring the previous image **and its matching complete pre-upgrade data**, including plugins and private media. Changing only a Docker tag after a database migration is not a verified rollback. See [updating](updating.md) and [Bunny deployment](deploy/bunny.md).

SQLite integrity checks run on private temporary copies of the database and persistent WAL/journal sidecars. SQLite can rebuild its ephemeral WAL index there; the stopped source remains unchanged. This also covers a committed WAL whose shared-memory index is absent.
