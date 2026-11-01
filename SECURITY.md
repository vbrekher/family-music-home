# Security policy

Do not post credentials, personal music, account details, private server addresses, family records or database files in public issues.

For a suspected vulnerability in this fork, use the repository **Security → Report a vulnerability** private reporting option when available. Include the affected release, a minimal synthetic reproduction and the impact. If private reporting is unavailable, do not disclose exploit details publicly: open a minimal issue asking the maintainer to arrange a private reporting channel, without sensitive material. No dedicated security email address or guaranteed response SLA is claimed.

For a vulnerability confined to upstream Navidrome, follow [Navidrome's current security policy](https://github.com/navidrome/navidrome/security/policy). State which fork/upstream revision is affected.

Keep server secrets outside Git, restrict library grants, use HTTPS remotely and restore-test complete encrypted backups before upgrades. If a credential leaks, revoke or rotate it with its provider and assess access before publishing a sanitized report. Never attach the leaked value.

See [Security & Privacy](docs/en/security.md) / [Безопасность](docs/ru/security.md) and [Backup & Restore](docs/en/backups.md). Current release acceptance distinguishes executed tests from physical-device and production checks; a source scan is not a guarantee of universal security.
