package family

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPrivateAttachmentLifecycleAndIdempotence(t *testing.T) {
	s, o, w, _ := fixture(t)
	s.MediaRoot = t.TempDir()
	photo := image.NewRGBA(image.Rect(0, 0, 8, 8))
	photo.Set(2, 2, color.RGBA{R: 255, A: 255})
	var b bytes.Buffer
	if e := png.Encode(&b, photo); e != nil {
		t.Fatal(e)
	}
	x, e := s.Save(o, Entry{Kind: "capsule", Title: "Photo memory", SharedWith: []string{"member"}, Body: Content{State: "draft", DateFrom: "2026-10-31", Timezone: "Europe/Kyiv"}})
	if e != nil {
		t.Fatal(e)
	}
	attachment, e := s.AddAttachment(o, x.ID, bytes.NewReader(b.Bytes()))
	if e != nil {
		t.Fatal(e)
	}
	if attachment.MIME != "image/png" {
		t.Fatal(attachment)
	}
	if _, _, e = s.Attachment(w, attachment.ID); !errors.Is(e, ErrDenied) {
		t.Fatal("draft photo leaked", e)
	}
	again, e := s.AddAttachment(o, x.ID, bytes.NewReader(b.Bytes()))
	if e != nil || again.ID != attachment.ID {
		t.Fatal("duplicate upload not idempotent", again, e)
	}
	x.Body.State = "published"
	x, e = s.Save(o, x)
	if e != nil {
		t.Fatal(e)
	}
	_, path, e := s.Attachment(w, attachment.ID)
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != attachment.SHA256 {
		t.Fatal("media hash mismatch")
	}
	if e = s.HideAttachment(w, attachment.ID, true); !errors.Is(e, ErrDenied) {
		t.Fatal("recipient changed source attachment", e)
	}
	if e = s.HideAttachment(o, attachment.ID, true); e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Attachment(w, attachment.ID); !errors.Is(e, ErrDenied) {
		t.Fatal("withdrawn photo leaked", e)
	}
	if e = s.HideAttachment(o, attachment.ID, false); e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Attachment(w, attachment.ID); e != nil {
		t.Fatal("photo restore failed", e)
	}
	if _, e = s.AddAttachment(o, x.ID, bytes.NewBufferString(`<svg onload="alert(1)"></svg>`)); !errors.Is(e, ErrInvalid) {
		t.Fatal("active upload accepted", e)
	}
	if _, e = s.AddAttachment(o, x.ID, bytes.NewReader(make([]byte, MaxAttachmentBytes+1))); !errors.Is(e, ErrInvalid) {
		t.Fatal("oversized upload accepted", e)
	}
}
func TestPendingMediaRecoveryKeepsCompleteFilesAndDropsOnlyUnfinishedPrivateData(t *testing.T) {
	s, o, _, _ := fixture(t)
	s.MediaRoot = t.TempDir()
	x, e := s.Save(o, Entry{Kind: "capsule", Title: "Recovery", Body: Content{State: "draft", DateFrom: "2026-10-31", Timezone: "UTC"}})
	if e != nil {
		t.Fatal(e)
	}
	data := []byte("private complete test bytes")
	hash := sha256.Sum256(data)
	key := "testcomplete0001"
	_, e = s.DB.Exec(`INSERT INTO family_attachment(id,entry_id,owner_id,mime,size,sha256,state,created_at) VALUES(?,?,?,'image/png',?,?,'pending',?)`, key, x.ID, "owner", len(data), hex.EncodeToString(hash[:]), time.Now().UTC().Add(-2*time.Hour).Format(time.RFC3339Nano))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(s.MediaRoot, key), data, 0o600); e != nil {
		t.Fatal(e)
	}
	if e = s.RecoverPendingMedia(context.Background(), time.Now()); e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Attachment(o, key); e != nil {
		t.Fatal("complete committed upload not recovered", e)
	}
}

func TestBoundedVoiceMessageProbe(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+16000))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint32(8000))
	binary.Write(&b, binary.LittleEndian, uint32(16000))
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(16000))
	b.Write(make([]byte, 16000))
	_, mime, e := normalizeMedia(context.Background(), b.Bytes(), "ffprobe")
	if e != nil || mime != "audio/wav" {
		t.Fatal("valid short voice rejected", mime, e)
	}
}

func TestPhotoThumbnailAuthorizationAndNormalization(t *testing.T) {
	s, o, w, _ := fixture(t)
	s.MediaRoot = t.TempDir()
	x, e := s.Save(o, Entry{Kind: "capsule", Title: "Large photo", SharedWith: []string{"member"}, Body: Content{State: "draft", DateFrom: "2026-10-31", Timezone: "UTC"}})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 3000, 1000)))
	a, e := s.AddAttachment(o, x.ID, bytes.NewReader(raw.Bytes()))
	if e != nil {
		t.Fatal(e)
	}
	_, path, e := s.Attachment(o, a.ID)
	if e != nil {
		t.Fatal(e)
	}
	f, _ := os.Open(path)
	cfg, _, e := image.DecodeConfig(f)
	f.Close()
	if e != nil || cfg.Width != 2048 {
		t.Fatal("photo not normalized", cfg, e)
	}
	if _, _, e = s.Thumbnail(w, a.ID); !errors.Is(e, ErrDenied) {
		t.Fatal("private thumbnail leaked", e)
	}
	x.Body.State = "published"
	x.Body.CoverAttachmentID = a.ID
	x, e = s.Save(o, x)
	if e != nil {
		t.Fatal(e)
	}
	thumb, path, e := s.Thumbnail(w, a.ID)
	if e != nil || thumb.Size > 32768 {
		t.Fatal(thumb, e)
	}
	f, _ = os.Open(path)
	cfg, _, e = image.DecodeConfig(f)
	f.Close()
	if e != nil || cfg.Width > 128 || cfg.Height > 128 {
		t.Fatal(cfg, e)
	}
	s.HideAttachment(o, a.ID, true)
	if _, _, e = s.Thumbnail(w, a.ID); !errors.Is(e, ErrDenied) {
		t.Fatal("cached thumbnail bypassed withdrawal", e)
	}
}

func TestVoiceFormatsAndDurationLimit(t *testing.T) {
	for _, format := range []string{"mp3", "ipod"} {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "voice."+format)
			command := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=8000", "-t", "1", "-ac", "1", "-f", format, path)
			if e := command.Run(); e != nil {
				t.Fatal(e)
			}
			raw, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			_, mime, e := normalizeMedia(context.Background(), raw, "ffprobe")
			if e != nil || mime == "" {
				t.Fatal("valid voice rejected", format, e)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "long.wav")
	if e := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "anullsrc=r=8000:cl=mono", "-t", "121", "-c:a", "pcm_s16le", path).Run(); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(path)
	if _, _, e := normalizeMedia(context.Background(), raw, "ffprobe"); !errors.Is(e, ErrInvalid) {
		t.Fatal("long voice accepted", e)
	}
}
