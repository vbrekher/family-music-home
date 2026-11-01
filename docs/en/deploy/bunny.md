# Self-host on Bunny Magic Containers

This is an optional remote deployment for your own account. The software is free; Bunny compute, registry and storage terms are your responsibility. First complete the [local installation](docker.md) and [restore rehearsal](../backups.md). There is no project-operated demo or registry containing your personal configuration.

## Prepare locally

1. Build from the accepted source on Docker Desktop: `docker build --platform linux/amd64 -f Dockerfile.family -t YOUR_PRIVATE_IMAGE:YOUR_IMMUTABLE_TAG .` Replace the example with your existing private registry/repository.
2. Authenticate using your registry's supported existing credentials, push the image and verify its remote manifest digest with Docker. Keep the exact image/digest and corresponding source release.
3. Bunny Magic Containers must be able to pull that private registry using an official registry binding and supported pull authorization. A local Docker Desktop login does not itself authorize Bunny to pull. Do not expose a temporary public registry, use an undocumented endpoint or assume a local OCI archive can be imported directly. If your existing credential cannot serve as pull authorization, record the access limitation before deploying.

## Configure your own instance

Use the official [Magic Containers documentation](https://bunny.net/docs/magic-containers) and dashboard. Add the exact private image, one replica and a **5 GB** persistent volume mounted at `/data`; container HTTP port **4533**. Use HTTPS at the public endpoint. Each replica has its own volume, so do not scale this SQLite deployment as if replicas shared a database. See Bunny's [persistent volume explanation](https://bunny.net/blog/magic-containers-now-supports-persistent-volumes-storage-that-survives-restarts-and-redeployments/).

Set `FMH_INSTALL_MODE=PUBLIC_SELF_HOST`. Keep sharing, external services and insight collection disabled unless deliberately needed. For the first administrator, use a random 32–96 character URL-safe `FMH_SETUP_TOKEN` in the platform secret store if you cannot privately read `/data/setup-token`. Open your own endpoint, enter that token, choose your own administrator credentials, then remove the optional setup secret after creation and restart. Do not install an existing person's database or predefined accounts.

Remote containers cannot read a folder on your laptop. Configure your own [Bunny library](../integrations/bunny.md) or supported external source; use Mac Sync only if you choose Bunny uploads. Store credentials as server secrets. Optional online Bunny backups require `FMH_BUNNY_BACKUPS=true`, a private zone write credential and a separately saved encryption key; see [backup limitations](../backups.md).

## Validate and update

Verify HTTPS `/ping`, both administrator/member logins, explicit library permissions, MP3/FLAC streaming and Range seeking. Create only synthetic family records for acceptance. Keep a complete encrypted backup of SQLite, plugin KV/packages and private media. Before upgrading an existing instance, use an official full-volume export/restore mechanism and verify the old image with its matching snapshot in isolation. Do not infer full-volume access from a volume-list API or use a SQLite-only snapshot as complete rollback evidence. If that mechanism is unavailable, keep the existing server unchanged and record a rollout blocker.

Never delete/recreate the persistent volume or reset users/secrets to update an image. Configure health checks, preserve the original replica/region settings and verify the deployed digest after an image-only update.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Unofficial Navidrome fork; GPLv3.
