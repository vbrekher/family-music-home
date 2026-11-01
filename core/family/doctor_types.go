package family

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const DoctorMaxItems = 200000
const DoctorMaxFindings = 1000
const DoctorMaxSourceBytes int64 = 1024 * 1024 * 1024
const DoctorMaxTotalBytes int64 = 2 * 1024 * 1024 * 1024

type DoctorTrack struct {
	CoverInvalid  bool    `json:"invalidCoverReference"`
	AlbumAudited  bool    `json:"-"`
	ID            string  `json:"id"`
	LibraryID     int     `json:"libraryId"`
	Library       string  `json:"library"`
	Title         string  `json:"title"`
	Artist        string  `json:"artist"`
	Album         string  `json:"album"`
	AlbumID       string  `json:"albumId"`
	Size          int64   `json:"size"`
	Version       string  `json:"version"`
	Suffix        string  `json:"container"`
	Codec         string  `json:"codec"`
	SampleRate    int     `json:"sampleRate"`
	BitDepth      *int    `json:"bitDepth"`
	Channels      int     `json:"channels"`
	Duration      float64 `json:"duration"`
	Year          int     `json:"year"`
	RecordingID   string  `json:"recordingId"`
	TrackNumber   int     `json:"trackNumber"`
	DiscNumber    int     `json:"discNumber"`
	HasCover      bool    `json:"hasCover"`
	Missing       bool    `json:"missing"`
	Available     bool    `json:"available"`
	FilenameTitle bool    `json:"filenameTitle"`
	PathHash      string  `json:"pathHash"`
}
type DoctorSettings struct {
	AfterID       string        `json:"afterId,omitempty"`
	MaxRows       int           `json:"maxRows"`
	SourceBudgets map[int]int64 `json:"sourceBudgets"`
	Fingerprint   bool          `json:"fingerprint"`
	Quality       bool          `json:"quality"`
	DecodeSeconds int           `json:"decodeSeconds"`
	Confirmed     bool          `json:"confirmed"`
	LibraryIDs    []int         `json:"libraryIds"`
}
type DoctorBudget struct {
	LibraryID int   `json:"libraryId"`
	Limit     int64 `json:"limitBytes"`
	Reserved  int64 `json:"reservedBytes"`
}
type DoctorResult struct {
	MeasuredCodec       string    `json:"measuredCodec,omitempty"`
	MeasuredSampleRate  int       `json:"measuredSampleRate,omitempty"`
	MeasuredChannels    int       `json:"measuredChannels,omitempty"`
	Worker              string    `json:"worker"`
	SHA256              string    `json:"sha256"`
	Decode              string    `json:"decode"`
	Duration            float64   `json:"duration"`
	Samples             int64     `json:"samples"`
	NearFullScale       int64     `json:"nearFullScale"`
	Peak                float64   `json:"peak"`
	SpectralBands       []float64 `json:"spectralBands,omitempty"`
	UpperBandRatio      *float64  `json:"upperBandRatio,omitempty"`
	BandwidthNote       string    `json:"bandwidthNote,omitempty"`
	Fingerprint         []uint32  `json:"fingerprint,omitempty"`
	FingerprintDuration float64   `json:"fingerprintDuration,omitempty"`
	Note                string    `json:"note,omitempty"`
}
type DoctorItem struct {
	Track      DoctorTrack  `json:"track"`
	State      string       `json:"state"`
	Lease      string       `json:"lease,omitempty"`
	LeaseUntil int64        `json:"leaseUntil,omitempty"`
	Attempts   int          `json:"attempts"`
	BytesRead  int64        `json:"bytesRead"`
	SHA256     string       `json:"sha256,omitempty"`
	Result     DoctorResult `json:"result"`
}
type DoctorJob struct {
	ID             string         `json:"id"`
	Mode           string         `json:"mode"`
	State          string         `json:"state"`
	Settings       DoctorSettings `json:"settings"`
	Cursor         string         `json:"cursor,omitempty"`
	Processed      int            `json:"processed"`
	CreatedAt      string         `json:"createdAt"`
	UpdatedAt      string         `json:"updatedAt"`
	Counts         map[string]int `json:"counts"`
	Budgets        []DoctorBudget `json:"budgets"`
	Findings       int            `json:"findingCount"`
	FindingsCapped bool           `json:"findingsCapped"`
}
type DoctorFinding struct {
	ID        int64  `json:"id"`
	TrackID   string `json:"trackId"`
	LibraryID int    `json:"libraryId"`
	Severity  string `json:"severity"`
	Code      string `json:"code"`
	Evidence  string `json:"evidence"`
	CreatedAt string `json:"createdAt"`
}
type Doctor struct {
	DB    *sql.DB
	Track func(context.Context, string) (DoctorTrack, error)
	Now   func() time.Time
}

func (d Doctor) now() time.Time {
	if d.Now != nil {
		return d.Now().UTC()
	}
	return time.Now().UTC()
}
func (d Doctor) authorize(ctx context.Context, key string) (DoctorJob, error) {
	u, e := actor(ctx)
	if e != nil {
		return DoctorJob{}, e
	}
	if !u.IsAdmin {
		return DoctorJob{}, ErrDenied
	}
	var x DoctorJob
	var settings string
	e = d.DB.QueryRowContext(ctx, `SELECT id,mode,state,settings,cursor,processed,created_at,updated_at FROM family_doctor_job WHERE id=? AND owner_id=?`, key, u.ID).Scan(&x.ID, &x.Mode, &x.State, &settings, &x.Cursor, &x.Processed, &x.CreatedAt, &x.UpdatedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return x, ErrDenied
	}
	if e != nil {
		return x, e
	}
	e = json.Unmarshal([]byte(settings), &x.Settings)
	return x, e
}
func (d Doctor) Get(ctx context.Context, key string) (DoctorJob, error) {
	x, e := d.authorize(ctx, key)
	if e != nil {
		return x, e
	}
	x.Counts = map[string]int{}
	x.Budgets = []DoctorBudget{}
	rows, e := d.DB.QueryContext(ctx, `SELECT state,count(*) FROM family_doctor_item WHERE job_id=? GROUP BY state`, key)
	if e != nil {
		return x, e
	}
	for rows.Next() {
		var state string
		var count int
		if e = rows.Scan(&state, &count); e != nil {
			rows.Close()
			return x, e
		}
		x.Counts[state] = count
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return x, e
	}
	rows, e = d.DB.QueryContext(ctx, `SELECT library_id,limit_bytes,reserved_bytes FROM family_doctor_budget WHERE job_id=? ORDER BY library_id`, key)
	if e != nil {
		return x, e
	}
	for rows.Next() {
		var b DoctorBudget
		if e = rows.Scan(&b.LibraryID, &b.Limit, &b.Reserved); e != nil {
			rows.Close()
			return x, e
		}
		x.Budgets = append(x.Budgets, b)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return x, e
	}
	if e = d.DB.QueryRowContext(ctx, `SELECT count(*) FROM family_doctor_finding WHERE job_id=?`, key).Scan(&x.Findings); e != nil {
		return x, e
	}
	if x.Mode == "deep" {
		x.Processed = 0
		for state, n := range x.Counts {
			if state != "queued" && state != "leased" && state != "fetching" && state != "downloaded" {
				x.Processed += n
			}
		}
	}
	var info, warnings, problems int
	if e = d.DB.QueryRowContext(ctx, `SELECT count(CASE WHEN severity='info' THEN 1 END),count(CASE WHEN severity='warning' THEN 1 END),count(CASE WHEN severity='error' THEN 1 END) FROM family_doctor_finding WHERE job_id=?`, key).Scan(&info, &warnings, &problems); e != nil {
		return x, e
	}
	x.FindingsCapped = x.Findings >= DoctorMaxFindings || info >= 500 || warnings >= 400 || problems >= 100
	return x, nil
}
func (d Doctor) List(ctx context.Context, offset int) ([]DoctorJob, error) {
	u, e := actor(ctx)
	if e != nil {
		return nil, e
	}
	if !u.IsAdmin {
		return nil, ErrDenied
	}
	if offset < 0 || offset > 10000 {
		return nil, ErrInvalid
	}
	rows, e := d.DB.QueryContext(ctx, `SELECT id FROM family_doctor_job WHERE owner_id=? ORDER BY created_at DESC,id LIMIT 25 OFFSET ?`, u.ID, offset)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var key string
		if e = rows.Scan(&key); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, key)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []DoctorJob{}
	for _, key := range ids {
		x, e := d.Get(ctx, key)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, nil
}
