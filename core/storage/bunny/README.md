# Direct Bunny read-only foundation

`New(Config)` returns `storage.Storage`; it does not register or activate itself.
It supports HTTPS regional Storage API origins, object listing, read-only file
handles, Seek via HTTP Range and one 256 KiB in-memory block per open file.
It requires a strong ETag and exact Content-Range, rejects redirects and
authentication/listing/transport errors, and sends If-Match on later blocks.
No object is mirrored to disk. The upstream Go TagLib extractor reads tags
through the same seekable file handles; metadata failures are returned.

Integration status and remaining limits:

- Composition-root registration is implemented in adapters/bunny; it validates the configured zone and returns safe configuration errors. Originally required: validate configured origin/zone/prefix/key once at application startup; obtain
  the `Storage`, then register a closure with `storage.Register("bunny", ...)`.
  Resolve URL host/prefix against this configured allowlist, rejecting mismatches.
  Never put an AccessKey in a URI or infer an arbitrary remote host from a track.
- Import/initialize the adapter from the composition root; do not register in
  `init` before configuration is available. The registry constructor cannot
  return errors, so invalid startup config should stop startup explicitly.
- Implemented: raw streamer and archives resolve Bunny paths instead of local `os.Open`, preserving Seek. Original requirement: selected library's
  MusicFS.Open and preserve io.Seeker. ffmpeg/probe need a separate safe input
  resolver (ideally short-lived authenticated loopback range proxy).
- Fix stored artwork paths, lyrics, playlists and inspection paths that still
  assume OS files. Do not advertise these as remote-ready yet.
- Actual Bunny origin passed credentialed Range/ETag and TagLib smoke tests.
  Missing strong ETag is intentionally unsupported rather than reading mixed
  object revisions. CDN FAQ Range support does not prove Storage API semantics.

Directory listings are cached for 2 seconds, bounded to 64 directories and
20,000 entries total. Failed/incomplete listings are never cached. Root Stat
validates the remote root listing. Transient 429/5xx GETs retry at most three
times with bounded delays.

Current limits: no global concurrency limiter,
request-scoped cancellation (fs.FS does not carry context), or watcher. HTTP
requests have a default 30-second timeout. Handles are single-caller as with
ordinary seek cursors. Listing is capped at 16 MiB; server-side stat is obtained
from the bounded parent listing cache. Large single directories beyond the entry
cap are intentionally uncached. No automatic registration or production changes are included.

Validation: `go test ./core/storage/bunny`, including actual remote-served MP3
fixture metadata, path escaping/traversal, Range/seek, ignored Range failure,
block reuse, changed ETag, redirect rejection and failed-listing propagation.
