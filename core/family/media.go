package family

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	draw "golang.org/x/image/draw"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/navidrome/navidrome/model/id"
)

const MaxAttachmentBytes = 5 * 1024 * 1024
const UserMediaQuota = 32 * 1024 * 1024
const FamilyMediaQuota = 128 * 1024 * 1024

var ErrQuota = errors.New("private family media quota reached")
var opaqueID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,80}$`)
var mediaSlots = make(chan struct{}, 1)

type Attachment struct {
	ID        string `json:"id"`
	EntryID   string `json:"entryId"`
	MIME      string `json:"mime"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	CreatedAt string `json:"createdAt"`
	Hidden    bool   `json:"hidden"`
}

func normalizeMedia(ctx context.Context, raw []byte, probe string) ([]byte, string, error) {
	if len(raw) == 0 || len(raw) > MaxAttachmentBytes {
		return nil, "", ErrInvalid
	}
	photo := bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")) || bytes.HasPrefix(raw, []byte("\xff\xd8\xff"))
	if photo {
		cfg, format, e := image.DecodeConfig(bytes.NewReader(raw))
		if e != nil || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 20000000 {
			return nil, "", ErrInvalid
		}
		img, _, e := image.Decode(bytes.NewReader(raw))
		if e != nil {
			return nil, "", ErrInvalid
		}
		if img.Bounds().Dx() > 2048 || img.Bounds().Dy() > 2048 {
			img = resizePhoto(img, 2048)
		}
		var out bytes.Buffer
		mime := "image/jpeg"
		if format == "png" {
			e = png.Encode(&out, img)
			mime = "image/png"
		} else {
			e = jpeg.Encode(&out, img, &jpeg.Options{Quality: 90})
		}
		if e != nil || out.Len() > MaxAttachmentBytes {
			return nil, "", ErrInvalid
		}
		return out.Bytes(), mime, nil
	}
	// Audio is probed exclusively from a bounded stdin pipe. The protocol whitelist
	// prevents a crafted container from opening local files or network references.
	if probe == "" {
		probe = "ffprobe"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, probe, "-v", "error", "-protocol_whitelist", "pipe", "-show_entries", "format=format_name,duration:stream=codec_type,codec_name,channels", "-of", "json", "-i", "pipe:0")
	cmd.Stdin = bytes.NewReader(raw)
	var out cappedOutput
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if e := cmd.Run(); e != nil {
		return nil, "", ErrInvalid
	}
	var info struct {
		Format struct {
			Name     string `json:"format_name"`
			Duration string `json:"duration"`
		}
		Streams []struct {
			Type     string `json:"codec_type"`
			Codec    string `json:"codec_name"`
			Channels int    `json:"channels"`
		}
	}
	if json.Unmarshal(out.Bytes(), &info) != nil || len(info.Streams) != 1 || info.Streams[0].Type != "audio" || info.Streams[0].Channels < 1 || info.Streams[0].Channels > 2 {
		return nil, "", ErrInvalid
	}
	mime := ""
	switch {
	case info.Format.Name == "mp3":
		mime = "audio/mpeg"
	case info.Format.Name == "wav":
		mime = "audio/wav"
	case strings.Contains(info.Format.Name, "mp4") || strings.Contains(info.Format.Name, "m4a"):
		mime = "audio/mp4"
	}
	if mime == "" {
		return nil, "", ErrInvalid
	}

	ffmpeg := filepath.Join(filepath.Dir(probe), "ffmpeg")
	args := []string{"-v", "error", "-nostdin", "-threads", "1", "-filter_threads", "1"}
	var input *bytes.Reader
	if mime == "audio/mp4" {
		// A generated private seekable file supports ordinary Voice Memos MP4 files;
		// external MOV data references and absolute aliases are explicitly disabled.
		file, e := os.CreateTemp("", ".navidrome-family-voice-*")
		if e != nil {
			return nil, "", e
		}
		defer os.Remove(file.Name())
		if _, e = file.Write(raw); e != nil {
			file.Close()
			return nil, "", e
		}
		if e = file.Close(); e != nil {
			return nil, "", e
		}
		args = append(args, "-protocol_whitelist", "file,pipe", "-f", "mov", "-enable_drefs", "0", "-use_absolute_path", "0", "-i", file.Name())
	} else {
		format := "mp3"
		if mime == "audio/wav" {
			format = "wav"
		}
		args = append(args, "-protocol_whitelist", "pipe", "-f", format, "-i", "pipe:0")
		input = bytes.NewReader(raw)
	}
	args = append(args, "-map", "0:a:0", "-t", "121", "-ac", "1", "-ar", "8000", "-f", "s16le", "pipe:1")
	decode := exec.CommandContext(ctx, ffmpeg, args...)
	if input != nil {
		decode.Stdin = input
	}
	var decoded sampleCounter
	decode.Stdout = &decoded
	decode.Stderr = io.Discard
	if e := decode.Run(); e != nil || decoded.Bytes <= 0 || decoded.Bytes > 120*8000*2 {
		return nil, "", ErrInvalid
	}
	return raw, mime, nil
}

type sampleCounter struct{ Bytes int64 }

func (c *sampleCounter) Write(b []byte) (int, error) { c.Bytes += int64(len(b)); return len(b), nil }

type cappedOutput struct{ bytes.Buffer }

func (b *cappedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 65536 {
		return 0, ErrInvalid
	}
	return b.Buffer.Write(p)
}
func (s Store) Attachments(ctx context.Context, entry string, includeHidden bool) ([]Attachment, error) {
	u, e := actor(ctx)
	if e != nil {
		return nil, e
	}
	x, e := s.Get(ctx, entry)
	if e != nil {
		return nil, e
	}
	if x.Kind != "capsule" && x.Kind != "letter" {
		return nil, ErrInvalid
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,entry_id,mime,size,sha256,created_at,hidden FROM family_attachment WHERE entry_id=? AND state='ready' AND (hidden=0 OR ?) ORDER BY created_at,id`, entry, includeHidden && x.OwnerID == u.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Attachment{}
	for rows.Next() {
		var a Attachment
		if e = rows.Scan(&a.ID, &a.EntryID, &a.MIME, &a.Size, &a.SHA256, &a.CreatedAt, &a.Hidden); e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s Store) AddAttachment(ctx context.Context, entry string, source io.Reader) (Attachment, error) {
	u, e := actor(ctx)
	if e != nil {
		return Attachment{}, e
	}
	x, e := s.Get(ctx, entry)
	if e != nil {
		return Attachment{}, e
	}
	if x.OwnerID != u.ID {
		return Attachment{}, ErrDenied
	}
	if x.Kind != "capsule" && x.Kind != "letter" || s.MediaRoot == "" {
		return Attachment{}, ErrInvalid
	}
	select {
	case mediaSlots <- struct{}{}:
		defer func() { <-mediaSlots }()
	case <-ctx.Done():
		return Attachment{}, ctx.Err()
	}
	raw, e := io.ReadAll(io.LimitReader(source, MaxAttachmentBytes+1))
	if e != nil {
		return Attachment{}, e
	}
	data, mime, e := normalizeMedia(ctx, raw, s.ProbePath)
	if e != nil {
		return Attachment{}, e
	}
	hash := sha256.Sum256(data)
	a := Attachment{ID: id.NewRandom(), EntryID: entry, MIME: mime, Size: int64(len(data)), SHA256: hex.EncodeToString(hash[:]), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return a, e
	}
	defer tx.Rollback()
	var owner, kind, body string
	if e = tx.QueryRowContext(ctx, `SELECT owner_id,kind,body FROM family_entry WHERE id=?`, entry).Scan(&owner, &kind, &body); e != nil {
		return a, ErrDenied
	}
	if owner != u.ID {
		return a, ErrDenied
	}
	content, e := decodeStored(body)
	if e != nil {
		return a, e
	}
	if kind == "letter" && content.State != "draft" {
		return a, ErrConflict
	}
	var existingID, existingState string
	e = tx.QueryRowContext(ctx, `SELECT id,state FROM family_attachment WHERE entry_id=? AND sha256=?`, entry, a.SHA256).Scan(&existingID, &existingState)
	if e == nil {
		if existingState != "ready" {
			return a, ErrConflict
		}
		tx.Rollback()
		v, _, e := s.Attachment(ctx, existingID)
		return v, e
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return a, e
	}
	var personal, total, count, allCount int64
	if e = tx.QueryRowContext(ctx, `SELECT coalesce(sum(CASE WHEN owner_id=? THEN size ELSE 0 END),0),coalesce(sum(size),0),count(CASE WHEN entry_id=? THEN 1 END),count(*) FROM family_attachment`, u.ID, entry).Scan(&personal, &total, &count, &allCount); e != nil {
		return a, e
	}
	if personal+a.Size > UserMediaQuota || total+a.Size > FamilyMediaQuota || count >= 24 || allCount >= 20000 {
		return a, ErrQuota
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO family_attachment(id,entry_id,owner_id,mime,size,sha256,state,created_at) VALUES(?,?,?,?,?,?,'pending',?)`, a.ID, entry, u.ID, a.MIME, a.Size, a.SHA256, a.CreatedAt); e != nil {
		return a, e
	}
	if e = tx.Commit(); e != nil {
		return a, e
	}
	renamed := false
	defer func() {
		if !renamed {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			s.DB.ExecContext(cleanup, `DELETE FROM family_attachment WHERE id=? AND state='pending'`, a.ID)
		}
	}()
	if e = os.MkdirAll(s.MediaRoot, 0o700); e != nil {
		return a, e
	}
	file, e := os.CreateTemp(s.MediaRoot, ".upload-"+a.ID+"-*")
	if e != nil {
		return a, e
	}
	defer os.Remove(file.Name())
	if _, e = file.Write(data); e == nil {
		e = file.Sync()
	}
	closeErr := file.Close()
	if e != nil {
		return a, e
	}
	if closeErr != nil {
		return a, closeErr
	}
	if e = os.Rename(file.Name(), filepath.Join(s.MediaRoot, a.ID)); e != nil {
		return a, e
	}
	renamed = true
	if _, e = s.DB.ExecContext(ctx, `UPDATE family_attachment SET state='ready' WHERE id=? AND state='pending'`, a.ID); e != nil {
		return a, e
	}
	return a, nil
}

// Attachment returns only a generated private path after the parent entry's
// authorization check, including draft/delivery rules and withdrawn media.
func (s Store) Attachment(ctx context.Context, key string) (Attachment, string, error) {
	u, e := actor(ctx)
	if e != nil {
		return Attachment{}, "", e
	}
	var a Attachment
	var owner string
	e = s.DB.QueryRowContext(ctx, `SELECT id,entry_id,owner_id,mime,size,sha256,created_at,hidden FROM family_attachment WHERE id=? AND state='ready'`, key).Scan(&a.ID, &a.EntryID, &owner, &a.MIME, &a.Size, &a.SHA256, &a.CreatedAt, &a.Hidden)
	if errors.Is(e, sql.ErrNoRows) {
		return a, "", ErrDenied
	}
	if e != nil {
		return a, "", e
	}
	if a.Hidden && owner != u.ID {
		return a, "", ErrDenied
	}
	if e = s.CanRead(ctx, a.EntryID); e != nil {
		return Attachment{}, "", e
	}
	if s.MediaRoot == "" || !opaqueID.MatchString(a.ID) {
		return Attachment{}, "", ErrInvalid
	}
	return a, filepath.Join(s.MediaRoot, a.ID), nil
}
func (s Store) HideAttachment(ctx context.Context, key string, hidden bool) error {
	u, e := actor(ctx)
	if e != nil {
		return e
	}
	a, _, e := s.Attachment(ctx, key)
	if e != nil {
		return e
	}
	res, e := s.DB.ExecContext(ctx, `UPDATE family_attachment SET hidden=? WHERE id=? AND entry_id=? AND owner_id=?`, hidden, key, a.EntryID, u.ID)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return ErrDenied
	}
	return nil
}
func (s Store) RecoverPendingMedia(ctx context.Context, now time.Time) error {
	if s.MediaRoot == "" {
		return nil
	}
	select {
	case mediaSlots <- struct{}{}:
		defer func() { <-mediaSlots }()
	default:
		return nil
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,size,sha256,created_at FROM family_attachment WHERE state='pending' AND created_at<? ORDER BY created_at LIMIT 20`, now.Add(-time.Minute).UTC().Format(time.RFC3339Nano))
	if e != nil {
		return e
	}
	type pending struct {
		id, hash, at string
		size         int64
	}
	list := []pending{}
	for rows.Next() {
		var p pending
		if e = rows.Scan(&p.id, &p.size, &p.hash, &p.at); e != nil {
			rows.Close()
			return e
		}
		list = append(list, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, p := range list {
		if !opaqueID.MatchString(p.id) {
			continue
		}
		path := filepath.Join(s.MediaRoot, p.id)
		data, e := os.ReadFile(path)
		if e == nil && int64(len(data)) == p.size {
			hash := sha256.Sum256(data)
			if hex.EncodeToString(hash[:]) == p.hash {
				if _, e = s.DB.ExecContext(ctx, `UPDATE family_attachment SET state='ready' WHERE id=? AND state='pending'`, p.id); e != nil {
					return e
				}
				continue
			}
		}
		at, e := time.Parse(time.RFC3339Nano, p.at)
		if e != nil {
			return e
		}
		if now.Sub(at) < time.Hour {
			continue
		}
		if _, e = s.DB.ExecContext(ctx, `DELETE FROM family_attachment WHERE id=? AND state='pending'`, p.id); e != nil {
			return e
		}
		os.Remove(path)
		temps, _ := filepath.Glob(filepath.Join(s.MediaRoot, ".upload-"+p.id+"-*"))
		for _, tmp := range temps {
			os.Remove(tmp)
		}
	}
	return nil
}

func resizePhoto(img image.Image, max int) image.Image {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w <= max && h <= max {
		return img
	}
	if w >= h {
		h = h * max / w
		w = max
	} else {
		w = w * max / h
		h = max
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	target := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(target, target.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.ApproxBiLinear.Scale(target, target.Bounds(), img, img.Bounds(), draw.Over, nil)
	return target
}

// Private thumbnails keep list views from loading multi-megabyte photographs.
// They are derived only from already authorized, normalized attachments.
func (s Store) Thumbnail(ctx context.Context, key string) (Attachment, string, error) {
	a, path, e := s.Attachment(ctx, key)
	if e != nil {
		return a, "", e
	}
	if !strings.HasPrefix(a.MIME, "image/") {
		return a, "", ErrInvalid
	}
	target := path + ".thumb"
	data, e := os.ReadFile(target)
	if errors.Is(e, os.ErrNotExist) {
		select {
		case mediaSlots <- struct{}{}:
			defer func() { <-mediaSlots }()
		case <-ctx.Done():
			return a, "", ctx.Err()
		}
		source, e := os.Open(path)
		if e != nil {
			return a, "", e
		}
		img, _, e := image.Decode(io.LimitReader(source, MaxAttachmentBytes+1))
		source.Close()
		if e != nil {
			return a, "", ErrInvalid
		}
		var b bytes.Buffer
		if e = jpeg.Encode(&b, resizePhoto(img, 128), &jpeg.Options{Quality: 75}); e != nil {
			return a, "", e
		}
		data = b.Bytes()
		if len(data) > 32768 {
			return a, "", ErrInvalid
		}
		f, e := os.CreateTemp(s.MediaRoot, ".thumb-*")
		if e != nil {
			return a, "", e
		}
		defer os.Remove(f.Name())
		if _, e = f.Write(data); e == nil {
			e = f.Sync()
		}
		f.Close()
		if e != nil {
			return a, "", e
		}
		if e = os.Rename(f.Name(), target); e != nil {
			return a, "", e
		}
	} else if e != nil {
		return a, "", e
	}
	hash := sha256.Sum256(data)
	a.SHA256 = hex.EncodeToString(hash[:])
	a.Size = int64(len(data))
	a.MIME = "image/jpeg"
	return a, target, nil
}
