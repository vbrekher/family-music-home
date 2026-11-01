# Developer guide

This is an unofficial GPLv3 fork of Navidrome. Start with [upstream differences](../../UPSTREAM_DIFFERENCES.md), [architecture](architecture.md), [contribution rules](../../CONTRIBUTING.md) and the [design system](../design/DESIGN_SYSTEM.md). Preserve inherited copyright and third-party notices.

The release build runs on local Docker Desktop for `linux/amd64`:

```sh
docker build --platform linux/amd64 -f Dockerfile.family -t family-music-home:local .
```

The pinned Dockerfile compiles the React bundle and Go server from this source. The Python runtime supports public first-run initialization and optional Bunny backup integration. Public startup does not require private deployment helpers, old Git history or a Bunny account.

UI dependencies are locked in `ui/package-lock.json`. Use `npm ci --legacy-peer-deps` in a local Node Docker environment, then `npm test -- --run --maxWorkers=1` and `npm run build`. The compatibility alias for JSONForms' retired UUID import is intentional. The local deepmerge security patch is documented in `ui/vendor/deepmerge/PATCH.md`; do not replace it with a vulnerable upstream version without reviewing the advisory.

Go requires CGO, a C toolchain and SQLite FTS5. In a local Go Docker environment install gcc/musl-dev/zlib-dev/git and run `go test -mod=readonly -p 1 -tags netgo,sqlite_fts5 ./...`. Build generated WASM test fixtures using their source Makefile when required; do not substitute private plugin files. Python unit tests use the requirements declared by their integration.

Use synthetic identities/music and disposable volumes. Never attach your production SQLite to development code. Test authorization for an unrelated member and a protected child, not only an administrator. A local passing test does not prove a physical NAS/phone or production rollout.

Changes to public releases pass the [safe export process](../../scripts/export_public_release.py): exact committed blobs, explicitly reviewed path/hash allowlist, private-value denylist supplied locally, secret scanning, clean independent build and a new public history. Do not mirror private development refs. No GitHub Actions, hosted builds or automatic reviewer assignments are required.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Unofficial Navidrome fork; GPLv3.

## Upstream regression data

The release does not redistribute upstream sample audio/artwork or published TLS test keys. Before running Go tests, explicitly run `python3 scripts/fetch-upstream-test-fixtures.py` in your local Docker development environment. It retrieves the original public Navidrome revision with a reviewed SHA-256 lock, refuses mismatches/overwrites and reconstructs only reviewed test symlinks inside the fixture directory. These files are development data, not demo media or production certificates. The application build and local synthetic demo do not require this download. Keep fetched fixtures out of public release archives and commits.

## Reproducible local test containers

```sh
docker build --platform linux/amd64 -f Dockerfile.test -t family-music-home-tests:local .
docker run --rm --platform linux/amd64 --cpus 2 --memory 2g family-music-home-tests:local
docker build --platform linux/amd64 --target ui -f Dockerfile.family -t family-music-home-ui:local .
docker run --rm --platform linux/amd64 --cpus 2 --memory 2g family-music-home-ui:local npx vitest run --maxWorkers=2 --testTimeout=60000
docker run --rm --platform linux/amd64 --entrypoint python3 -v "$PWD:/src:ro" -w /src family-music-home:local -m unittest discover -s tests/bunny_ops -v
```

The Go test image fetches the reviewed upstream regression fixtures and builds WASM test plugins from source. It runs as UID 502, so unreadable-file tests remain meaningful. Keep it local; fetched sample media and public fixture keys are not release assets. The final operations command assumes you built `family-music-home:local` above. Public exports omit private deployment-guard tests that depend on owner-only scripts. The advanced inherited cross-compilation recipe is `Dockerfile.upstream`; the default `Dockerfile` uses the same protected self-host profile as `Dockerfile.family`.

## Real clean-install acceptance

With local Docker Desktop and Python 3 on the host, use a **new disposable volume** and the original six-album demo. The runner refuses unlabelled/non-loopback containers and existing installations without a first-run token. It creates only synthetic accounts, verifies MP3/FLAC Range and library revocation, restarts the app, and optionally performs a complete encrypted stopped-volume backup/restore. Generated credentials and the temporary backup key stay in memory. Do not attach personal data.

```sh
mkdir -p demo-music
docker run --rm --platform linux/amd64 --entrypoint python3 -v "$PWD:/src:ro" -v "$PWD/demo-music:/demo" family-music-home:local /src/scripts/create-demo-library.py /demo
docker run -d --platform linux/amd64 --name family-clean-check --label net.family-music-home.disposable=true --cpus 2 --memory 1g -p 127.0.0.1:45549:4533 -v family-clean-check-data:/data -v "$PWD/demo-music:/music:ro" family-music-home:local
python3 tests/selfhost_acceptance.py --container family-clean-check --restore-check
```

The original test container/volume remain available for inspection. The temporary restored clone is removed. Choose fresh names for another run. The runtime uses the image's default user, matching Compose. If you select a custom UID/GID, first initialize ownership of the new data volume; Docker does not make a root-owned empty volume writable automatically.
