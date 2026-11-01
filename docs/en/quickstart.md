# Start with your own music

[English](quickstart.md) · [Русский](../ru/quickstart.md)

You need Docker Desktop running and a folder containing music you own or have permission to use. No Bunny account, registry account or cloud service is required. The first source build can take several minutes; downloading dependencies depends on your connection.

1. Download the source release or clone `https://github.com/aitishnyk/family-music-home`. Open a terminal in that folder.
2. Set your music folder, then start the server:

   ```sh
   export FMH_MUSIC_DIR="$HOME/Music/Navidrome"
   mkdir -p "$FMH_MUSIC_DIR"
   docker compose -f compose.family.yml up -d --build
   ```

3. Read your one-time setup token locally. Do not paste it into an issue or chat:

   ```sh
   docker compose -f compose.family.yml exec music cat /data/setup-token
   ```

4. Open [your local setup page](http://127.0.0.1:4533). Choose your administrator username and a unique password of at least 12 characters. Enter the setup token and create the account. There are no preinstalled user passwords.
5. Let the scanner index the folder. Open Albums, choose an album and press Play. FLAC may be transcoded if your browser cannot play its codec directly.
6. Open Settings → Users, create a regular member and explicitly review that person's library access. Use Child accounts for protected child profiles. Keep unneeded sharing disabled.
7. Choose your own theme from the settings menu. Explore [the eleven family features](features/index.md), then read [backup and restore](backups.md).

The music folder is mounted read-only. Removing the container does not remove the original music. The `family-data` named volume holds accounts, history and private attachments: do not use `down -v` unless you intentionally want to destroy that application data and have a verified backup.

This default listens only on your computer. To serve other devices, first arrange HTTPS and authentication using the [Docker deployment guide](deploy/docker.md). Do not expose an unfinished setup to the Internet.

If setup fails, check `docker compose -f compose.family.yml logs --tail=80 music`. Do not share raw logs containing personal information. An existing database with accounts does not reopen first-run signup. The token file is removed on the next restart after registration; keeping a copy does not permit a second first administrator.
