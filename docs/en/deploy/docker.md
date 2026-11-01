# Docker installation

[English](docker.md) · [Русский](../../ru/deploy/docker.md)

For a first installation, follow [Quick Start](../quickstart.md). The supplied `compose.family.yml` builds `Dockerfile.family`, mounts your music read-only, keeps application state in a named volume and binds the web port to localhost. Bunny is optional. First source builds download dependencies; subsequent builds reuse local Docker caches.

## What you control

- `FMH_MUSIC_DIR`: the existing host music directory. Do not use the application data directory for original audio.
- `/data`: accounts, SQLite, playlists/history, private media and plugin state. Keep it across container replacement.
- `127.0.0.1:4533`: the default local web address. The sample container has a 1 GiB memory limit and two CPUs.
- First setup: your own administrator name/password and a locally readable setup token. Existing accounts do not reopen signup.

To test with original demonstration audio instead of your own music, see [Local Demo](../local-demo.md).

## Other devices and HTTPS

1. Complete local setup and verify an administrator and a regular member with explicit library permissions.
2. Arrange HTTPS on infrastructure you already operate, with a valid certificate and appropriate authenticated access. A plain localhost address is not an Internet deployment.
3. Change only the intended port/proxy binding. Keep authentication, setup-token protection and read-only music mounts. Do not publish server secrets in Compose or Git.
4. Test a member login, permitted playback, seek, a denied library and logout from another browser session. Physical phones and client applications require their own test; desktop viewport emulation does not prove device compatibility.

Use a container UID/GID that can read your music and write its own data volume. If overriding the image's user, prepare volume ownership first. Do not loosen all host-file permissions to work around an ownership error.

## Routine commands

```sh
docker compose -f compose.family.yml ps
docker compose -f compose.family.yml logs --tail=80 music
docker compose -f compose.family.yml stop music
docker compose -f compose.family.yml start music
```

Use [full encrypted backup and restore](../backups.md) before updating. `docker compose down` removes the application container/network; `down -v` also destroys named-volume state and is not a routine uninstall command. Your read-only host music directory remains separate. [Updating](../updating.md) explains version changes and rollback.

The fork's release acceptance targets `linux/amd64`, built locally with Docker Desktop on macOS. Source availability is not a claim that every upstream platform or client has been physically tested by this fork.
