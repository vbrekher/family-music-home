# Connect a home NAS

[English](nas.md) · [Русский](../../ru/integrations/nas.md)

A NAS is storage hardware, not a protocol. Choose its [SFTP](sftp.md) or [HTTPS WebDAV](webdav.md) service, or an approved [S3-compatible endpoint](s3.md). For a local Docker server, an existing read-only OS-mounted share can instead be mounted as /music. Do not copy a multi-terabyte library into /data.

1. On the NAS, create a read-only music account and restrict it to the intended directory.
2. Confirm that the application host can reach the NAS through your existing secure network. A cloud container cannot reach your home LAN merely because it has a NAS address. Use an already-approved VPN/private transport or a properly secured supported origin; this product does not create a tunnel or a new server.
3. Configure the selected protocol, verified certificate/SSH fingerprint and explicit approved network ranges. Never expose SMB directly to the Internet.
4. Test, add the library, scan and grant a member access. Play and seek a permitted recording.
5. Take the NAS offline safely, verify the catalog remains and reconnect. Do not call this test a physical-NAS PASS unless it was actually performed on that hardware.

The application never deletes NAS originals. Removing the library association removes catalog entries, not the share. Source availability depends on the NAS, network, credentials and any transport you operate. Physical-device status is reported separately from local protocol-fixture tests.
