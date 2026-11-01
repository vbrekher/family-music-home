# Lyrics Plugin

Lyrics lookup for compatible OpenSubsonic clients. The standard WebUI does not gain a lyrics panel. Embedded lyrics take priority; provider coverage and synchronized text vary.

Pinned integration: **Navidrome Lyrics Plugin v8.1.0**, `nd-lyrics.ndp`, MIT. [Tagged upstream source](https://github.com/J0R6IT0/navidrome-lyrics-plugin/tree/v8.1.0). Exact original, derived-package and WASM SHA-256 values are in `integrations/plugins/plugins.lock.json`; licenses are in `integrations/plugins/licenses/`.

## Install deliberately

1. Back up `/data`, including existing plugin packages and KV databases. Never overwrite a personal plugin installation blindly.
2. From source run `python3 scripts/stage-family-plugins.py plugin-staging` in your local Docker Python environment. This downloads all three pinned upstream packages, verifies their checksums and produces restricted derived manifests in a staging directory. It does not install or enable them on a server.
3. Review the lockfile and permissions before copying the chosen `nd-lyrics.ndp` into your own persistent plugin folder, normally `/data/plugins`. Set `ND_PLUGINS_ENABLED=true`, `ND_PLUGINS_FOLDER=/data/plugins` and `ND_PLUGINS_AUTORELOAD=false`; restart, open **Plugins** as administrator and explicitly approve only the intended capabilities. Use `integrations/plugins/local.env.example` as a reviewed reference, not a replacement for existing settings.
4. Open the plugin's configuration form, review defaults and save. Test using a library you are allowed to access. Do not grant filesystem or unrelated network access to obtain a result.
5. Restart and verify that configuration and results persist. KV files belong in backups; transient caches can be rebuilt.

## Explicit metadata opt-in

Keep built-in external integrations and telemetry disabled. To use the reviewed optional family plugins, explicitly set the following together with the package settings above:

```dotenv
ND_ENABLEEXTERNALSERVICES=false
ND_ENABLEFAMILYMETADATAPLUGINS=true
ND_AGENTS=apple-music,coverartarchive
ND_LYRICSPRIORITY=embedded,nd-lyrics
```

Only installed and approved plugins run. Select just the agents you install; missing providers do not become built-in fallbacks. This enables optional metadata requests to the declared plugin domains, not scrobbling or insights.

## Network and limits

The staged manifest permits **lrclib.net**. Metadata about the requested recording/artist may leave your server when you enable the plugin. No paid provider subscription or new project account is required by these pinned defaults. The lyrics integration is restricted to LRCLIB and cannot write sidecar files; do not enable provider URLs outside the allowlist as a workaround.

If no result appears, verify capability approval, outbound DNS/HTTPS, matching tags/identifiers and upstream provider availability. Keep local embedded metadata usable when a provider is offline. To stop remote lookups, disable the plugin, restart and verify music still plays. Removing a package does not delete source music, but retain its KV database until you deliberately discard its state.

Provider success is distinct from loading a WASM package. Consult release acceptance for exact live/protocol coverage; no claim is made that every recording has lyrics, artist metadata or artwork.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Unofficial Navidrome fork; GPLv3.
