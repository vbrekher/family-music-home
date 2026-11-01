# Third-party notices

The application is based on Navidrome (GPLv3). Preserve LICENSE and existing copyright notices. Dependency versions are locked in `go.mod`, `go.sum` and `ui/package-lock.json`; these components retain their respective licenses. A source release does not relicense third-party dependencies under a different license.

## Optional metadata plugins

The staging tool downloads upstream packages only after SHA-256 verification, keeps their WASM bytes unchanged and derives restricted manifests. It does not grant filesystem write permission to lyrics. Versions, upstream/staged/WASM digests and host capabilities are recorded in `integrations/plugins/plugins.lock.json`.

| Component | Version | License | Corresponding upstream source |
|---|---|---|---|
| Navidrome Lyrics Plugin | v8.1.0 | MIT | [tagged source](https://github.com/J0R6IT0/navidrome-lyrics-plugin/tree/v8.1.0) |
| Apple Music Metadata Plugin | v0.2.0 | GPLv3 | [tagged source](https://github.com/navidrome/apple-music-plugin/tree/v0.2.0) |
| Cover Art Archive Plugin | v1.0.0 | MIT | [tagged source](https://github.com/sunsetroute1/navidrome-coverartarchive-plugin/tree/v1.0.0) |

License texts are included in `integrations/plugins/licenses/` and in each staged package. The upstream tagged LICENSE files were checked during release preparation: [Lyrics MIT](https://raw.githubusercontent.com/J0R6IT0/navidrome-lyrics-plugin/v8.1.0/LICENSE), [Apple Music GPLv3](https://raw.githubusercontent.com/navidrome/apple-music-plugin/v0.2.0/LICENSE), [Cover Art Archive MIT](https://raw.githubusercontent.com/sunsetroute1/navidrome-coverartarchive-plugin/v1.0.0/LICENSE).

No `.ndp` binary is required for a basic local install. If redistributing a plugin binary, supply its license and the exact corresponding source/build instructions, including your manifest changes; a link to this fork's application source alone is insufficient for a separately built plugin.

## Build/runtime components

Dockerfiles pin Node.js, Go and the Navidrome runtime image by digest. Their upstream licenses, and the licenses of Alpine packages, remain applicable. FFmpeg configuration determines whether a binary is LGPL or GPL; inspect the actual image's `ffmpeg -L` and `ffmpeg -buildconf` before redistributing it. Chromaprint is a separate diagnostic tool, not proof that two recordings are identical. Its source and license must accompany any redistributed diagnostic binary as required by its license.

The source release contains build recipes, not a promise that every optional third-party service remains available. Apple Music, MusicBrainz, Cover Art Archive and LRCLIB have their own provider policies. Optional provider requests are disclosed in the integration guides.

## Artwork, fonts and screenshots

Original demo recordings and covers must be identified in the demo asset provenance file. Public screenshots use only synthetic accounts and demonstration data. User music and private family media are never release assets. Existing upstream artwork/fonts/icons retain their original notices. No commercial album artwork is licensed by this project merely because the application can display it.

## Local dependency hardening

The UI includes a readable, MIT-licensed downstream patch of deepmerge 4.3.1 in `ui/vendor/deepmerge`. Its local identifier is `4.3.2-fmh.1`, not an upstream release. The patch excludes prototype-control property names from both input-copy paths; see its PATCH.md, retained license and regression tests. `decode-uri-component` is pinned to upstream 0.5.1 for CVE-2026-118664; affected older UUID dependencies are overridden to a compatible patched CommonJS release. Workbox CLI is a build tool and is listed as a development dependency rather than a browser runtime dependency.

The install-time `ui/bin/patch-jsonforms-uuid.cjs` compatibility patch checks the SHA-256 of the pinned JSONForms 2.5.2 CommonJS renderer, then replaces the retired `uuid/v1` subpath with the supported `require("uuid").v1` export. JSONForms retains its original MIT license. The Vite alias covers the upstream ESM bundle.

Upstream regression media and public TLS fixture keys are not redistributed in the clean release. Developers can explicitly retrieve the exact upstream fixtures using `scripts/fetch-upstream-test-fixtures.py` and its SHA-256 lock. This is separate from the original CC0 synthetic demo; do not use regression media as promotional artwork/music or deploy published test keys.

The bundled Unbounded font retains Copyright 2022 The Unbounded Project Authors (https://github.com/googlefonts/unbounded) and the SIL Open Font License 1.1, included at `ui/public/fonts/OFL.txt`. It is not relicensed under GPL. Font file SHA-256: `0bc1485a9593ca9f452ca78951c81e553a9e361ecd49e07c37da8ea1d27dacc1`. The upstream OFL notice was verified against the official Google Fonts repository.

Runtime dependency inventories and retained npm notices are in `docs/release/dependency-licenses.go.json`, `docs/release/dependency-licenses.npm.json` and `docs/release/third-party-npm-licenses.txt`. Missing npm archive notices are supplemented from reviewed upstream sources with hashes/provenance in `integrations/licenses/npm/sources.lock.json`. The Go analyzer does not recursively inspect assembly/C dependencies; TagLib retains LGPL-2.1 and its pinned binding source, and base-image libraries retain their own notices. Cryptography is pinned to official PyPI 50.0.2 artifacts with transitive dependencies and download SHA-256 verification.
