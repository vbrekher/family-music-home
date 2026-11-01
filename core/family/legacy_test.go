package family

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"image"
	"image/png"
	"io"
	"sort"
	"strings"
	"testing"
	"time"
)

func legacyFixture(t *testing.T) (Legacy, context.Context, context.Context) {
	s, _, _, _ := fixture(t)
	s.MediaRoot = t.TempDir()
	_, e := s.DB.Exec(`INSERT INTO user VALUES('owner0001'),('member00001'); CREATE TABLE family_legacy_export(id TEXT PRIMARY KEY,user_id TEXT UNIQUE REFERENCES user(id),options TEXT,created_at TEXT,expires_at TEXT)`)
	if e != nil {
		t.Fatal(e)
	}
	s.UserExists = func(_ context.Context, id string) bool { return id == "owner0001" || id == "member00001" }
	s.IsAdministrator = func(_ context.Context, id string) bool { return id == "owner0001" }
	o := request.WithUser(context.Background(), model.User{ID: "owner0001", IsAdmin: true})
	w := request.WithUser(context.Background(), model.User{ID: "member00001"})
	return Legacy{Store: s}, o, w
}
func legacyTestFiles(t *testing.T, raw []byte) map[string][]byte {
	t.Helper()
	z, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if e != nil {
		t.Fatal(e)
	}
	out := map[string][]byte{}
	for _, f := range z.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		out[f.Name], e = io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	return out
}
func legacyTestArchive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	names := []string{}
	for name := range files {
		if name != "checksums.sha256" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var sums strings.Builder
	for _, name := range names {
		h := sha256.Sum256(files[name])
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(h[:]), name)
	}
	files["checksums.sha256"] = []byte(sums.String())
	names = append(names, "checksums.sha256")
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, name := range names {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write(files[name]); e != nil {
			t.Fatal(e)
		}
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestLegacyOwnAndAllowedSharedPrivacyMediaAndReadOnlyVerification(t *testing.T) {
	l, o, w := legacyFixture(t)
	s := l.Store
	own, e := s.Save(o, Entry{Kind: "capsule", Title: "Own memory 🐰", Body: Content{State: "published", DateFrom: "2026-10-31", Timezone: "Europe/Kyiv", Description: "Unicode family history", References: []Reference{{Kind: "track", ID: "track0001"}}}})
	if e != nil {
		t.Fatal(e)
	}
	var photo bytes.Buffer
	png.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	a, e := s.AddAttachment(o, own.ID, &photo)
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Save(w, Entry{Kind: "capsule", Title: "WIFE_PRIVATE_SENTINEL", Body: Content{State: "published", DateFrom: "2026-10-31", Timezone: "UTC"}})
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Save(w, Entry{Kind: "shelf", Title: "Explicit shared", SharedWith: []string{"owner0001"}, Body: Content{References: []Reference{}}})
	if e != nil {
		t.Fatal(e)
	}
	j, e := l.Prepare(o, LegacyOptions{Scope: "personal", Attachments: true, Archived: true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = l.Job(w, j.ID); e != ErrDenied {
		t.Fatal("cross-account export", e)
	}
	m, e := l.Manifest(o, j.ID)
	if e != nil || len(m.Entries) != 1 || len(m.Media) != 1 {
		t.Fatal(m, e)
	}
	shared, e := l.Prepare(o, LegacyOptions{Scope: "allowed-shared", Attachments: true, Archived: true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = l.Job(o, j.ID); e != ErrDenied {
		t.Fatal("old descriptor not revoked")
	}
	m, e = l.Manifest(o, shared.ID)
	if e != nil || len(m.Entries) != 2 {
		t.Fatal(m, e)
	}
	raw, _ := json.Marshal(m)
	if bytes.Contains(raw, []byte("WIFE_PRIVATE_SENTINEL")) {
		t.Fatal("administrator exported other adult private content")
	}
	var archive bytes.Buffer
	if e = l.Write(o, m, &archive); e != nil {
		t.Fatal(e)
	}
	verified, e := VerifyLegacy(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if e != nil || !verified.Verified || verified.Encrypted || verified.Entries != 2 {
		t.Fatal(verified, e)
	}
	files := legacyTestFiles(t, archive.Bytes())
	h := sha256.Sum256(files[m.Media[0].File])
	if hex.EncodeToString(h[:]) != a.SHA256 {
		t.Fatal("restored attachment hash differs")
	}

	if !bytes.Contains(files["collections/"+own.ID+".m3u8"], []byte("../music/track0001")) {
		t.Fatal("portable relative reference missing")
	}
	var n int
	if e = s.DB.QueryRow(`SELECT count(*) FROM family_entry`).Scan(&n); e != nil || n != 3 {
		t.Fatal("verifier changed live records", n, e)
	}
	l.Now = func() time.Time { return time.Now().Add(11 * time.Minute) }
	if _, e = l.Job(o, shared.ID); e != ErrDenied {
		t.Fatal("expired export accessible", e)
	}
}
func TestLegacyVerifierRejectsTraversalTamperUnknownSchemaAndAllocationBomb(t *testing.T) {
	l, o, _ := legacyFixture(t)
	_, e := l.Store.Save(o, Entry{Kind: "shelf", Title: "Safe history", Body: Content{References: []Reference{}}})
	if e != nil {
		t.Fatal(e)
	}
	j, e := l.Prepare(o, LegacyOptions{Scope: "personal"})
	if e != nil {
		t.Fatal(e)
	}
	m, e := l.Manifest(o, j.ID)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = l.Write(o, m, &b); e != nil {
		t.Fatal(e)
	}
	original := b.Bytes()
	for _, kind := range []string{"traversal", "tamper", "schema", "count", "huge-member"} {
		t.Run(kind, func(t *testing.T) {
			files := legacyTestFiles(t, original)
			var raw []byte
			switch kind {
			case "traversal":
				files["../escape"] = []byte("no")
				raw = legacyTestArchive(t, files)
			case "tamper":
				files["README.md"] = []byte("changed")
				var out bytes.Buffer
				z := zip.NewWriter(&out)
				for name, data := range files {
					w, _ := z.Create(name)
					w.Write(data)
				}
				z.Close()
				raw = out.Bytes()
			case "schema":
				var value map[string]any
				json.Unmarshal(files["manifest.json"], &value)
				value["schemaVersion"] = 99
				files["manifest.json"], _ = json.Marshal(value)
				raw = legacyTestArchive(t, files)
			case "count":
				raw = append([]byte(nil), original...)
				index := bytes.LastIndex(raw, []byte{'P', 'K', 5, 6})
				binary.LittleEndian.PutUint16(raw[index+8:], 65535)
				binary.LittleEndian.PutUint16(raw[index+10:], 65535)
			case "huge-member":
				files["attachments/synthetic0001.png"] = make([]byte, MaxAttachmentBytes+1)
				raw = legacyTestArchive(t, files)
			}
			if v, e := VerifyLegacy(bytes.NewReader(raw), int64(len(raw))); e == nil || v.Verified {
				t.Fatal("unsafe archive accepted", kind, v)
			}
		})
	}
}
