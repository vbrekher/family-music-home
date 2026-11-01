package family

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
)

func safeLegacyPath(name string) bool {
	if name == "" || len(name) > 180 || name != path.Clean(name) || strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\:\x00\r\n") || name == ".." || strings.HasPrefix(name, "../") {
		return false
	}
	for _, r := range name {
		if r < 32 || r > 126 {
			return false
		}
	}
	return true
}

type LegacyVerification struct {
	Verified      bool   `json:"verified"`
	SchemaVersion int    `json:"schemaVersion"`
	Entries       int    `json:"entries"`
	Files         int    `json:"files"`
	Bytes         int64  `json:"uncompressedBytes"`
	Encrypted     bool   `json:"encrypted"`
	Note          string `json:"note"`
}

// VerifyLegacy never extracts members or changes live family/catalog data.
// The small EOCD preflight bounds central-directory allocation before zip.NewReader.
func VerifyLegacy(source io.ReaderAt, size int64) (LegacyVerification, error) {
	bad := func() (LegacyVerification, error) { return LegacyVerification{}, ErrInvalid }
	if size < 22 || size > LegacyMaxArchive {
		return bad()
	}
	tailSize := min(size, int64(65557))
	tail := make([]byte, tailSize)
	if _, e := source.ReadAt(tail, size-tailSize); e != nil {
		return bad()
	}
	i := bytes.LastIndex(tail, []byte{'P', 'K', 5, 6})
	if i < 0 || i+22 > len(tail) {
		return bad()
	}
	eocd := tail[i:]
	if len(eocd) != 22+int(binary.LittleEndian.Uint16(eocd[20:])) {
		return bad()
	}
	count := int(binary.LittleEndian.Uint16(eocd[10:]))
	centralSize := int64(binary.LittleEndian.Uint32(eocd[12:]))
	centralOffset := int64(binary.LittleEndian.Uint32(eocd[16:]))
	if binary.LittleEndian.Uint16(eocd[4:]) != 0 || binary.LittleEndian.Uint16(eocd[6:]) != 0 || int(binary.LittleEndian.Uint16(eocd[8:])) != count || count < 3 || count > LegacyMaxFiles || centralSize > 8*1024*1024 || centralOffset+centralSize > size-22 {
		return bad()
	}
	z, e := zip.NewReader(source, size)
	if e != nil || len(z.File) != count {
		return bad()
	}
	files := map[string]*zip.File{}
	digests := map[string]string{}
	var total int64
	var manifestRaw, checksums []byte
	for _, f := range z.File {
		if !safeLegacyPath(f.Name) || f.Flags&1 != 0 || !f.Mode().IsRegular() || files[f.Name] != nil {
			return bad()
		}
		files[f.Name] = f
		limit := int64(MaxAttachmentBytes)
		if f.Name == "manifest.json" {
			limit = LegacyMaxMetadata
		} else if f.Name == "checksums.sha256" {
			limit = 2 * 1024 * 1024
		} else if strings.HasPrefix(f.Name, "playlists/") {
			limit = LegacyMaxMetadata
		} else if f.Name == "README.md" || strings.HasPrefix(f.Name, "memories/") || strings.HasPrefix(f.Name, "collections/") {
			limit = 131072
		} else if !strings.HasPrefix(f.Name, "attachments/") {
			return bad()
		}
		if f.UncompressedSize64 > uint64(limit) {
			return bad()
		}
		total += int64(f.UncompressedSize64)
		if total > LegacyMaxBytes {
			return bad()
		}
		r, e := f.Open()
		if e != nil {
			return bad()
		}
		hash := sha256.New()
		var keep bytes.Buffer
		dest := io.Writer(hash)
		if f.Name == "manifest.json" || f.Name == "checksums.sha256" {
			dest = io.MultiWriter(hash, &keep)
		}
		n, e := io.Copy(dest, io.LimitReader(r, limit+1))
		closeErr := r.Close()
		if e != nil || closeErr != nil || n > limit || uint64(n) != f.UncompressedSize64 {
			return bad()
		}
		digests[f.Name] = hex.EncodeToString(hash.Sum(nil))
		if f.Name == "manifest.json" {
			manifestRaw = keep.Bytes()
		}
		if f.Name == "checksums.sha256" {
			checksums = keep.Bytes()
		}
	}
	if len(manifestRaw) == 0 || len(checksums) == 0 || files["README.md"] == nil {
		return bad()
	}
	declared := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(checksums), "\n"), "\n") {
		if len(line) < 67 || line[64:66] != "  " {
			return bad()
		}
		digest, name := line[:64], line[66:]
		if !validSHA(digest) || !safeLegacyPath(name) || declared[name] != "" || name == "checksums.sha256" || digests[name] != digest {
			return bad()
		}
		declared[name] = digest
	}
	if len(declared) != len(files)-1 {
		return bad()
	}
	decoder := json.NewDecoder(bytes.NewReader(manifestRaw))
	decoder.DisallowUnknownFields()
	var m LegacyManifest
	if decoder.Decode(&m) != nil || decoder.Decode(new(any)) != io.EOF || m.SchemaVersion != 1 || m.AudioIncluded || !opaqueID.MatchString(m.AccountID) || len(m.Entries) > LegacyMaxEntries || validLegacyOptions(m.Options) != nil {
		return bad()
	}
	expected := map[string]bool{"manifest.json": true, "README.md": true, "checksums.sha256": true}
	entries := map[string]bool{}
	for _, x := range m.Entries {
		if !opaqueID.MatchString(x.ID) || entries[x.ID] || !validKind(x.Kind) {
			return bad()
		}
		entries[x.ID] = true
		expected["memories/"+x.ID+".md"] = true
		expected["collections/"+x.ID+".m3u8"] = true
	}
	for _, a := range m.Media {
		suffix := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "audio/mpeg": ".mp3", "audio/wav": ".wav", "audio/mp4": ".m4a"}[a.MIME]
		if !entries[a.EntryID] || !opaqueID.MatchString(a.ID) || suffix == "" || a.File != "attachments/"+a.ID+suffix || expected[a.File] || a.Size < 1 || a.Size > MaxAttachmentBytes || files[a.File] == nil || int64(files[a.File].UncompressedSize64) != a.Size || digests[a.File] != a.SHA256 {
			return bad()
		}
		expected[a.File] = true
	}
	for _, p := range m.Playlists {
		if !opaqueID.MatchString(p.ID) || len(p.References) > 5000 || expected["playlists/"+p.ID+".m3u8"] {
			return bad()
		}
		expected["playlists/"+p.ID+".m3u8"] = true
	}
	if len(expected) != len(files) {
		return bad()
	}
	for name := range expected {
		if files[name] == nil {
			return bad()
		}
	}
	return LegacyVerification{Verified: true, SchemaVersion: 1, Entries: len(m.Entries), Files: len(files), Bytes: total, Encrypted: false, Note: fmt.Sprintf("Read-only verification of %d declared records. SHA-256 proves byte integrity, not authorship. The ZIP is unencrypted; no import or live database change was performed. Restore complete accounts and media using the documented encrypted server snapshot.", len(m.Entries))}, nil
}
