# Security and privacy

Your server contains private listening history, letters, memories and possibly photographs or voice attachments. Keep `/data`, backups and plugin KV stores private. Use HTTPS for remote access, strong unique passwords and ordinary member accounts for listening. Restrict Docker to loopback until you deliberately configure a trusted HTTPS proxy.

The public first run requires a randomly generated setup token read from your own server. It is not a default password and does not grant a later login. After the first administrator is created, further first-admin requests are refused. Restart removes the token file once accounts exist. Never share it in support tickets.

New public members receive no automatic library access. Administrators grant libraries explicitly. Family sharing references music; it does not grant audio access. Protected children have additional restrictions. Review recipients before sending a letter or publishing a memory. Legacy exports contain personal data and must be protected like backups.

External storage passwords and keys belong in server secret configuration, never browser forms, Git or public logs. Connection forms refer to a secret by environment variable name. TLS/SSH host verification and private-network protections must remain enabled. Remove a source configuration rather than deleting original music.

External metadata and lyrics are optional. Enabling a plugin sends selected metadata and requests to its declared providers. Review capability permissions and keep plugin versions pinned. The public Docker profile disables external services, sharing, Gravatar and insight collection by default; explicitly enabled plugins still have their own network behavior.

Report vulnerabilities privately using [SECURITY.md](../../SECURITY.md), with minimal synthetic reproduction. Do not include real tokens, music, private addresses, SQLite files or family records. GPL availability does not make your hosted server or its data public.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Unofficial Navidrome fork; GPLv3.
