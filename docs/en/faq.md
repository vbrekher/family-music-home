# Frequently asked questions

**Is it free?** The application is GPLv3 free software with no premium modules, advertising or required subscription. You pay your own optional hosting/storage bills.

**Is this official Navidrome?** No. This is an unofficial fork. Standard library/player/OpenSubsonic behavior is inherited; see [differences](../../UPSTREAM_DIFFERENCES.md).

**Do I need Bunny?** No. Start with a local read-only music folder and Docker. Remote storage is optional.

**Where do I get music?** Supply recordings you are entitled to use. The synthetic [local demo](local-demo.md) creates original test recordings and artwork; commercial music is not included.

**Does Family Sync delete my cloud music?** No. It is add/update-only from Mac to Bunny; deleting a local file does not delete the remote object. See [its limits](integrations/mac-sync.md).

**Is a Legacy ZIP my backup?** No. It is a personal history export. Recovery requires the [complete encrypted backup](backups.md), matching image and required configuration.

**Can I use a phone or NAS?** Compatible clients and supported remote protocols are implemented. Physical hardware tests must be distinguished from browser/protocol tests. There is no shared online demo server.

**Why no lyrics in the ordinary WebUI?** The plugin provides the server's lyrics capability. Display and synchronized lyrics depend on the client; installing a plugin does not add a new WebUI lyrics panel.

**Can I modify or commercially redistribute it?** GPLv3 permits modification and redistribution subject to its terms, including corresponding-source obligations. Preserve notices and third-party licenses.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Unofficial Navidrome fork; GPLv3.
