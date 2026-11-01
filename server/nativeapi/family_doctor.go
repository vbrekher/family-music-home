package nativeapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/core/family"
	"github.com/navidrome/navidrome/core/storage"
	"github.com/navidrome/navidrome/db"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
)

func boundedCatalogText(s string) string {
	r := []rune(s)
	if len(r) > 100 {
		return string(r[:100]) + "…"
	}
	return s
}
func (api *Router) doctorTrack(ctx context.Context, key string) (family.DoctorTrack, error) {
	m, e := api.ds.MediaFile(ctx).Get(key)
	if e != nil {
		return family.DoctorTrack{}, e
	}
	return api.doctorTrackDTO(ctx, *m), nil
}
func (api *Router) doctorTrackDTO(ctx context.Context, m model.MediaFile) family.DoctorTrack {
	hash := sha256.Sum256([]byte(m.AbsolutePath()))
	base := strings.TrimSuffix(filepath.Base(m.Path), filepath.Ext(m.Path))
	return family.DoctorTrack{CoverInvalid: m.HasCoverArt && m.ImageAbsent, ID: m.ID, LibraryID: m.LibraryID, Library: boundedCatalogText(m.LibraryName), Title: boundedCatalogText(m.Title), Artist: boundedCatalogText(m.Artist), Album: boundedCatalogText(m.Album), AlbumID: m.AlbumID, Size: m.Size, Version: m.UpdatedAt.UTC().Format(time.RFC3339Nano), Suffix: m.Suffix, Codec: m.Codec, SampleRate: m.SampleRate, BitDepth: m.BitDepth, Channels: m.Channels, Duration: float64(m.Duration), Year: m.Year, RecordingID: m.MbzRecordingID, TrackNumber: m.TrackNumber, DiscNumber: m.DiscNumber, HasCover: m.HasCoverArt, Missing: m.Missing, Available: api.familyAvailable(ctx, m.LibraryPath, m.Missing), FilenameTitle: base == m.Title, PathHash: hex.EncodeToString(hash[:])}
}
func (api *Router) familyDoctor() family.Doctor {
	return family.Doctor{DB: db.Db(), Track: api.doctorTrack}
}
func doctorAdmin(w http.ResponseWriter, r *http.Request) bool {
	u, ok := request.UserFrom(r.Context())
	if !ok || !u.IsAdmin {
		familyError(w, family.ErrDenied)
		return false
	}
	return true
}
func (api *Router) addFamilyDoctorRoutes(r chi.Router) {
	r.Route("/doctor", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if doctorAdmin(w, r) {
					next.ServeHTTP(w, r)
				}
			})
		})
		r.Get("/tracks/{id}", func(w http.ResponseWriter, r *http.Request) {
			x, e := api.doctorTrack(r.Context(), chi.URLParam(r, "id"))
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, x)
		})
		r.Get("/jobs", func(w http.ResponseWriter, r *http.Request) {
			xs, e := api.familyDoctor().List(r.Context(), familyOffset(r))
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, xs)
		})
		r.Post("/jobs", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Mode     string                `json:"mode"`
				Settings family.DoctorSettings `json:"settings"`
				TrackIDs []string              `json:"trackIds"`
			}
			if !familyDecode(w, r, &body) {
				return
			}
			x, e := api.familyDoctor().Create(r.Context(), body.Mode, body.Settings, body.TrackIDs)
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, x)
		})
		r.Get("/jobs/{job}", func(w http.ResponseWriter, r *http.Request) {
			x, e := api.familyDoctor().Get(r.Context(), chi.URLParam(r, "job"))
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, x)
		})
		r.Post("/jobs/{job}/control", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Action string `json:"action"`
				Retry  bool   `json:"retry"`
			}
			if !familyDecode(w, r, &body) {
				return
			}
			x, e := api.familyDoctor().Control(r.Context(), chi.URLParam(r, "job"), body.Action, body.Retry)
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, x)
		})
		r.Post("/jobs/{job}/budgets", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Limits map[int]int64 `json:"limits"`
			}
			if !familyDecode(w, r, &body) {
				return
			}
			x, e := api.familyDoctor().RaiseBudgets(r.Context(), chi.URLParam(r, "job"), body.Limits)
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, x)
		})
		r.Get("/jobs/{job}/items", func(w http.ResponseWriter, r *http.Request) {
			x, e := api.familyDoctor().Items(r.Context(), chi.URLParam(r, "job"), familyOffset(r))
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, x)
		})
		r.Get("/jobs/{job}/findings", func(w http.ResponseWriter, r *http.Request) {
			x, e := api.familyDoctor().Findings(r.Context(), chi.URLParam(r, "job"), r.URL.Query().Get("severity"), familyOffset(r))
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, x)
		})
		r.Get("/jobs/{job}/duplicates", func(w http.ResponseWriter, r *http.Request) {
			x, e := api.familyDoctor().Duplicates(r.Context(), chi.URLParam(r, "job"))
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, x)
		})
		r.Post("/jobs/{job}/step", api.doctorMetadataStep)
		r.Post("/jobs/{job}/claim", func(w http.ResponseWriter, r *http.Request) {
			x, e := api.familyDoctor().Claim(r.Context(), chi.URLParam(r, "job"))
			if e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, x)
		})
		r.Get("/jobs/{job}/items/{track}/content", api.doctorContent)
		r.Post("/jobs/{job}/items/{track}/result", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Lease  string              `json:"lease"`
				Result family.DoctorResult `json:"result"`
			}
			if !familyDecode(w, r, &body) {
				return
			}
			if e := api.familyDoctor().Complete(r.Context(), chi.URLParam(r, "job"), chi.URLParam(r, "track"), body.Lease, body.Result); e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, map[string]bool{"saved": true})
		})
		r.Get("/jobs/{job}/export", api.doctorExport)
		r.Post("/jobs/{job}/remove-report", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Confirmed bool `json:"confirmed"`
			}
			if !familyDecode(w, r, &body) {
				return
			}
			if e := api.familyDoctor().RemoveReport(r.Context(), chi.URLParam(r, "job"), body.Confirmed); e != nil {
				familyError(w, e)
				return
			}
			familyJSON(w, map[string]bool{"removed": true})
		})
	})
}
func (api *Router) doctorMetadataStep(w http.ResponseWriter, r *http.Request) {
	d := api.familyDoctor()
	key := chi.URLParam(r, "job")
	job, e := d.Get(r.Context(), key)
	if e != nil {
		familyError(w, e)
		return
	}
	if job.Mode != "metadata" || job.State != "queued" && job.State != "active" {
		familyError(w, family.ErrConflict)
		return
	}
	max := min(200, job.Settings.MaxRows-job.Processed)
	if max <= 0 {
		familyError(w, family.ErrConflict)
		return
	}
	filters := sq.And{sq.Gt{"media_file.id": job.Cursor}}
	if len(job.Settings.LibraryIDs) > 0 {
		filters = append(filters, sq.Eq{"media_file.library_id": job.Settings.LibraryIDs})
	}
	tracks, e := api.ds.MediaFile(r.Context()).GetAll(model.QueryOptions{Filters: filters, Sort: "id", Max: max})
	if e != nil {
		familyError(w, e)
		return
	}
	page := []family.DoctorTrack{}
	albumFindings := []family.DoctorFinding{}
	albums := map[string]bool{}
	audited := map[string]bool{}
	checked, e := db.Db().QueryContext(r.Context(), `SELECT album_id FROM family_doctor_album_audit WHERE job_id=?`, key)
	if e != nil {
		familyError(w, e)
		return
	}
	for checked.Next() {
		var album string
		if e = checked.Scan(&album); e != nil {
			checked.Close()
			familyError(w, e)
			return
		}
		audited[album] = true
	}
	e = checked.Err()
	checked.Close()
	if e != nil {
		familyError(w, e)
		return
	}
	for _, m := range tracks {
		dto := api.doctorTrackDTO(r.Context(), m)
		dto.AlbumAudited = true
		page = append(page, dto)
		if albums[m.AlbumID] || audited[m.AlbumID] {
			continue
		}
		albums[m.AlbumID] = true
		rows, e := db.Db().QueryContext(r.Context(), `SELECT disc_number,count(*),count(DISTINCT track_number),min(track_number),max(track_number) FROM media_file WHERE album_id=? AND library_id=? AND missing=0 GROUP BY disc_number LIMIT 100`, m.AlbumID, m.LibraryID)
		if e != nil {
			familyError(w, e)
			return
		}
		for rows.Next() {
			var disc, count, distinct, low, high int
			if e = rows.Scan(&disc, &count, &distinct, &low, &high); e != nil {
				rows.Close()
				familyError(w, e)
				return
			}
			if low > 0 && high > distinct && len(albumFindings) < 400 {
				albumFindings = append(albumFindings, family.DoctorFinding{TrackID: m.ID, LibraryID: m.LibraryID, Severity: "warning", Code: "numbering_gap", Evidence: fmt.Sprintf("Disc %d has numbering gaps in the catalog. It may be incomplete or intentionally numbered this way; no storage probe was performed.", disc)})
			}
			if low > 0 && count > distinct && len(albumFindings) < 400 {
				albumFindings = append(albumFindings, family.DoctorFinding{TrackID: m.ID, LibraryID: m.LibraryID, Severity: "warning", Code: "repeated_track_number", Evidence: fmt.Sprintf("Disc %d repeats track numbers. This may be multiple encodings or tag ambiguity, not proven duplicate audio.", disc)})
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			familyError(w, e)
			return
		}
	}
	x, e := d.AcceptMetadataPage(r.Context(), key, page, len(page) < max, albumFindings)
	if e != nil {
		familyError(w, e)
		return
	}
	familyJSON(w, x)
}

type doctorReader struct {
	ctx    context.Context
	reader io.Reader
	check  func() error
	last   time.Time
}

func (r *doctorReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	if time.Since(r.last) > 100*time.Millisecond {
		if e := r.check(); e != nil {
			return 0, e
		}
		r.last = time.Now()
	}
	return r.reader.Read(b)
}
func (api *Router) doctorContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		familyError(w, family.ErrInvalid)
		return
	}
	if r.Header.Get("Range") != "" {
		familyError(w, family.ErrInvalid)
		return
	}
	key, trackID, lease := chi.URLParam(r, "job"), chi.URLParam(r, "track"), r.Header.Get("X-ND-Doctor-Lease")
	d := api.familyDoctor()
	track, e := d.BeginFetch(r.Context(), key, trackID, lease)
	if e != nil {
		familyError(w, e)
		return
	}
	// Reuse the same catalog-authorized read-only source as ordinary raw playback.
	m, e := api.ds.MediaFile(r.Context()).Get(trackID)
	if e != nil {
		d.FinishFetch(r.Context(), key, trackID, lease, 0, "", false)
		familyError(w, e)
		return
	}
	if m.Size != track.Size || m.LibraryID != track.LibraryID || m.UpdatedAt.UTC().Format(time.RFC3339Nano) != track.Version {
		d.FinishFetch(r.Context(), key, trackID, lease, 0, "", false)
		familyError(w, family.ErrConflict)
		return
	}
	var file io.ReadCloser
	if model.IsRemotePath(m.AbsolutePath()) {
		file, e = storage.OpenRemote(m.AbsolutePath())
	} else {
		file, e = os.Open(m.AbsolutePath())
	}
	if e != nil {
		d.FinishFetch(r.Context(), key, trackID, lease, 0, "", false)
		familyError(w, fmt.Errorf("source unavailable"))
		return
	}
	defer file.Close()
	if statter, ok := file.(interface{ Stat() (fs.FileInfo, error) }); ok {
		info, e := statter.Stat()
		if e != nil || info.Size() != track.Size {
			d.FinishFetch(r.Context(), key, trackID, lease, 0, "", false)
			familyError(w, family.ErrConflict)
			return
		}
	}
	streamCtx, cancelStream := context.WithCancel(r.Context())
	defer cancelStream()
	stop := context.AfterFunc(streamCtx, func() { file.Close() })
	defer stop()
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-streamCtx.Done():
				return
			case <-ticker.C:
				if d.LeaseActive(streamCtx, key, trackID, lease) != nil {
					cancelStream()
					return
				}
			}
		}
	}()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", fmt.Sprint(track.Size))
	hash := sha256.New()
	reader := &doctorReader{ctx: streamCtx, reader: file, check: func() error { return d.LeaseActive(r.Context(), key, trackID, lease) }}
	n, copyErr := io.CopyN(io.MultiWriter(w, hash), reader, track.Size)
	digest := hex.EncodeToString(hash.Sum(nil))
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
	defer cancel()
	_ = d.FinishFetch(finishCtx, key, trackID, lease, n, digest, copyErr == nil && n == track.Size)
}
func (api *Router) doctorExport(w http.ResponseWriter, r *http.Request) {
	d := api.familyDoctor()
	key := chi.URLParam(r, "job")
	job, e := d.Get(r.Context(), key)
	if e != nil {
		familyError(w, e)
		return
	}
	findings := []family.DoctorFinding{}
	for offset := 0; offset < family.DoctorMaxFindings; offset += 100 {
		page, e := d.Findings(r.Context(), key, "", offset)
		if e != nil {
			familyError(w, e)
			return
		}
		findings = append(findings, page...)
		if len(page) < 100 {
			break
		}
	}
	duplicates, e := d.Duplicates(r.Context(), key)
	if e != nil {
		familyError(w, e)
		return
	}
	selections := []family.DoctorItem{}
	if job.Mode == "deep" {
		for offset := 0; offset < 100; offset += 50 {
			page, e := d.Items(r.Context(), key, offset)
			if e != nil {
				familyError(w, e)
				return
			}
			selections = append(selections, page...)
			if len(page) < 50 {
				break
			}
		}
	}
	w.Header().Set("Content-Disposition", `attachment; filename="library-doctor.json"`)
	familyJSON(w, map[string]any{"schemaVersion": 1, "job": job, "findings": findings, "duplicates": duplicates, "selections": selections, "readOnly": true, "note": "Diagnostic metadata only. No original audio was modified. Spectrum and metadata cannot prove mastering or ancestry."})
}
