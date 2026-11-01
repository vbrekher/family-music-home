# A private local demonstration

This demo runs on your own computer. It does not connect to a shared server and contains no personal music.

1. Install Docker Desktop and download the source. Follow [Quick Start](quickstart.md) through obtaining the source, but choose a separate empty folder for demo music.
2. From the source directory generate original short recordings and procedural covers:

```sh
mkdir -p demo-music
docker build --platform linux/amd64 -f Dockerfile.family-doctor -t family-doctor:local .
docker run --rm --entrypoint python3 -v "$PWD:/src:ro" -v "$PWD/demo-music:/demo" family-doctor:local /src/scripts/create-demo-library.py /demo
```

The generator refuses to overwrite existing audio. Use a fresh directory for another run. It produces six fictional albums, twelve short MP3/FLAC recordings and a provenance file; generated audio/artwork are CC0. The generator source is GPLv3.

3. Set `FMH_MUSIC_DIR` in `.env` to the absolute `demo-music` path and start the public Docker profile as described in Quick Start. Use a separate Compose project name if another instance exists: `docker compose -p fmh-demo -f compose.family.yml up -d --build`. Change the published host port first if 4533 is occupied.
4. Read this instance's setup token, create your own fictional administrator, then a member. Explicitly grant the demo library to the member.
5. Create a shelf, a fictional memory and a music request using the [feature guides](features/index.md). Do not import personal backups for screenshots.

Stop with `docker compose -p fmh-demo -f compose.family.yml down`. This retains its data volume and generated music. Delete those separately only when you deliberately no longer need the demo. The project supplies no publicly shared credentials or online demo.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Unofficial Navidrome fork; GPLv3.
