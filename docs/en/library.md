# Add and browse your music

Family Music Home indexes audio tags. It does not purchase, download or redistribute commercial music for you. Use recordings you are allowed to store and play.

For a local folder, follow [Quick Start](quickstart.md). The container reads `/music`; mount it read-only. Copy an album into the host music folder, then use **Scan** in the administrator menu if the scheduled scan has not run. Wait for completion before diagnosing an empty album. A scan reads metadata; it does not upload your local music to Bunny.

Open **Albums**, **Artists**, **Songs** or **Genres** in the sidebar. Use search, filters and sort controls to narrow the list. Album and artist grouping follows tags such as Album Artist, Album, Title, track and disc number. Correct bad tags in your original files with your preferred tag editor and rescan. A filename alone does not guarantee correct grouping.

Select a heart for a favourite; use the rating control when enabled. These preferences belong to your account. Artwork is read from supported embedded or folder covers; remote artwork services are optional and disclose metadata to their providers when enabled.

For remote collections use [S3](integrations/s3.md), [SFTP](integrations/sftp.md), [WebDAV](integrations/webdav.md), [NAS](integrations/nas.md) or [Bunny](integrations/bunny.md). A missing source must be repaired before streaming can resume. Avoid repeatedly deleting and recreating it: temporary disconnection is not a reason to erase a catalog.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Unofficial Navidrome fork; GPLv3.
