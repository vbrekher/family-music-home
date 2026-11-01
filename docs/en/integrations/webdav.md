# HTTPS WebDAV storage

[English](../../en/integrations/webdav.md) · [Русский](../../ru/integrations/webdav.md)

Use this when music should stay on your existing storage. The application indexes metadata and streams authorized files on demand. It does not mirror the complete music collection into the database volume. Only a server administrator can configure an origin or add its library.

## Prepare the connection

Enable HTTPS WebDAV with a valid certificate and a read-only account. Use an HTTPS origin without credentials, query or path; put the music directory in root instead. Plain HTTP and untrusted TLS are refused.

The referenced JSON secret contains username and password. If your provider places WebDAV under a long path, that relative path belongs in root, for example remote.php/dav/files/example/Music, not in endpoint.

Create an operator-managed sources.json with a configuration like this. Replace every example value with your own connection settings.

```json
{
  "sources": [
    {
      "id": "home-webdav",
      "type": "webdav",
      "endpoint": "https://dav.example.com",
      "root": "music",
      "secret_env": "NAVIDROME_SOURCE_HOME_MUSIC",
      "read_only": true,
      "enabled": true
    }
  ]
}
```

Mount the file read-only and set ND_EXTERNALSOURCESFILE to its container path. Supply NAVIDROME_SOURCE_HOME_MUSIC through the platform’s secret store, not a committed Compose file. Restart to load approved origins. Credentials and arbitrary endpoints cannot be submitted by regular library users.

## Add and verify

1. Open External Music Sources as an administrator. Find the approved source ID.
2. Choose Test connection. Fix credential, certificate, host-key or network failures before scanning.
3. Choose Add library, then Scan. In Users, explicitly grant the intended member access to this library.
4. Sign in as that member. Find an indexed album, play a recording and seek within it. Try a file format supported by your browser or transcoder.
5. Disable the source and confirm its catalog remains; playback is temporarily unavailable. Re-enable and test/scan after reconnection.
6. Use Remove association only when you intend to remove the indexed library and its catalog entries. The confirmation does not authorize deletion of remote originals. Remove or disable the operator configuration separately if it should no longer be connectable.

## Network and failure behavior

Private/local destinations are denied unless the server operator explicitly approves the required network ranges. Do not disable TLS, host-key or DNS/IP checks to make a connection pass. DNS is checked at connection time, redirects are not followed, and only configured source IDs are resolved.

An outage is not an empty successful scan: a failed source keeps its previous catalog. Check source status, read-only credentials and reachability, then reconnect. Provider listing limits, request charges and storage egress still apply. Browser playback can cause Range requests; transcoding may read more audio. Physical NAS and third-party devices require their own real acceptance; protocol fixtures do not prove hardware compatibility.

See [NAS networking](nas.md), [accounts](../users.md), [troubleshooting](../troubleshooting.md), and [the screenshot gallery](../gallery.md#sources).
