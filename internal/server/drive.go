package server

import (
	"context"
	"github.com/google/uuid"
	"github.com/msvens/mimage/metadata"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"github.com/msvens/mphotos/internal/gdrive"
	"go.uber.org/zap"
	"google.golang.org/api/drive/v3"
	"math"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	fileFields = "id, name, kind, mimeType, md5Checksum, createdTime"
)

type DriveFile struct {
	CreatedTime time.Time
	Id          string `json:"id"`
	Kind        string `json:"kind"`
	Md5Checksum string `json:"md5Checksum"`
	MimeType    string `json:"mimeType"`
}

type DriveFiles struct {
	Length int          `json:"length"`
	Files  []*DriveFile `json:"files,omitempty"`
}

func (s *mserver) handleAddDrivePhotos(r *http.Request) (interface{}, error) {
	return addDrivePhotos(r.Context(), s)
}

func (s *mserver) handleSearchDrive(r *http.Request) (interface{}, error) {
	name := r.URL.Query().Get("name")
	id := r.URL.Query().Get("id")
	if files, err := searchDriveFiles(s, id, name); err != nil {
		return nil, err
	} else {
		return toDriveFiles(files), nil
	}
}

func (s *mserver) handleDrive(_ *http.Request) (interface{}, error) {
	if files, err := listDriveFiles(s); err != nil {
		return nil, err
	} else {
		return toDriveFiles(files), nil
	}
}

func (s *mserver) handleAuthenticatedDrive(_ *http.Request) (interface{}, error) {
	return AuthUser{s.isGoogleConnected()}, nil
}

func (s *mserver) handleDisconnectDrive(_ *http.Request) (interface{}, error) {
	// Remove token file
	if err := os.Remove(s.tokenFile); err != nil && !os.IsNotExist(err) {
		s.l.Errorw("error removing token file", zap.Error(err))
		return nil, InternalError(err.Error())
	}

	// Clear Google services
	s.ds = nil
	s.ms = nil

	s.l.Info("Disconnected from Google services")

	// Return authentication status (false = disconnected)
	return AuthUser{false}, nil
}

// DriveCheck is the "what's new" overview: how many not-yet-imported images and
// videos are in the Drive folder. Whether video is supported at all is a server
// capability (see GET /api/capabilities), not a Drive-check concern.
type DriveCheck struct {
	Images int `json:"images"`
	Videos int `json:"videos"`
}

func (s *mserver) handleCheckDrive(_ *http.Request) (interface{}, error) {
	images, err := checkDrivePhotos(s)
	if err != nil {
		return nil, err
	}
	check := DriveCheck{Images: len(images)}
	if s.videoEnabled {
		videos, err := checkDriveVideos(s)
		if err != nil {
			return nil, err
		}
		check.Videos = len(videos)
	}
	return check, nil
}

func addDrivePhoto(ctx context.Context, s *mserver, f *drive.File) (bool, error) {
	if s.pg.Photo.HasMd5(f.Md5Checksum) {
		return false, nil
	}
	photo := dao.Photo{}
	photo.Id = uuid.New()
	photo.SourceId = f.Id
	photo.Md5 = f.Md5Checksum
	photo.Source = dao.SourceGoogle
	photo.FileName = photo.Id.String() + ".jpg" //same naming convention for gdrive and local; non-jpeg is converted
	if t, err := gdrive.ParseTime(f.CreatedTime); err == nil {
		photo.SourceDate = t
	}
	photo.UploadDate = time.Now()

	// Download into a staged temp file, then detect the format by content: Drive's
	// reported mime can be wrong, and this also gates out anything we can't decode.
	stagedPath := config.PhotoFilePath(config.Original, photo.Id.String()+".import")
	if _, err := s.ds.Download(ctx, f.Id, stagedPath); err != nil {
		_ = os.Remove(stagedPath)
		s.l.Errorw("error downloading img", zap.Error(err))
		return false, err
	}
	srcFormat, err := metadata.DetectFormatFile(stagedPath)
	if err != nil || !srcFormat.Supported() {
		_ = os.Remove(stagedPath)
		s.l.Debugw("skipping unsupported drive image", "name", f.Name, "mime", f.MimeType)
		return false, nil
	}
	if err := s.finalizeImport(&photo, stagedPath, srcFormat); err != nil {
		return false, err
	}
	s.l.Infow("added img", "driveId", photo.Id)
	return true, nil
}

func addDrivePhotos(ctx context.Context, s *mserver) (*DriveFiles, error) {
	fl, err := listDriveFiles(s)
	if err != nil {
		return nil, err
	}

	var files []*drive.File
	for _, f := range fl {
		added, err := addDrivePhoto(ctx, s, f)
		if err != nil {
			return nil, err
		}
		if added {
			files = append(files, f)
		}
	}
	return toDriveFiles(files), nil
}

func checkDrivePhotos(s *mserver) ([]*drive.File, error) {
	fl, err := listDriveFiles(s)
	if err != nil {
		return nil, err
	}
	var ret []*drive.File
	for _, f := range fl {
		if !s.pg.Photo.HasMd5(f.Md5Checksum) {
			ret = append(ret, f)
		}
	}
	return ret, nil
}

func listDriveFiles(s *mserver) ([]*drive.File, error) {
	if u, err := s.pg.User.Get(); err != nil {
		return nil, InternalError("user not found")
	} else if u.DriveFolderId == "" {
		return nil, NotFoundError("Drive folder has not been set")
	} else {
		return searchDriveFiles(s, u.DriveFolderId, "")
	}
}

func searchDriveFiles(s *mserver, id string, name string) ([]*drive.File, error) {
	return searchDriveByMime(s, id, name, "image/")
}

// searchDriveByMime lists files in the folder whose mime type starts with
// mimePrefix ("image/" or "video/"). Format support is still decided per file at
// import time by content, since Drive's reported mime can be wrong.
func searchDriveByMime(s *mserver, id, name, mimePrefix string) ([]*drive.File, error) {
	if s.ds == nil {
		return nil, UnauthorizedError("No Drive Service Connected")
	}
	if name != "" {
		if f, err := s.ds.GetByName(name, true, false, fileFields); err != nil {
			return nil, err
		} else {
			id = f.Id
		}
	}
	query := gdrive.NewQuery().Parents().In(id).And().MimeType().Contains(mimePrefix).TrashedEq(false)
	return s.ds.SearchAll(query, fileFields)
}

func toDriveFile(file *drive.File) *DriveFile {
	df := DriveFile{
		Id:          file.Id,
		Kind:        file.Kind,
		Md5Checksum: file.Md5Checksum,
		MimeType:    file.MimeType,
	}
	df.CreatedTime, _ = gdrive.ParseTime(file.CreatedTime)
	return &df
}

func toDriveFiles(files []*drive.File) *DriveFiles {
	ret := DriveFiles{Length: len(files)}
	if ret.Length > 0 {
		for _, f := range files {

			ret.Files = append(ret.Files, toDriveFile(f))
		}
	}
	return &ret
}

// async
func (s *mserver) handleScheduleDriveJob(_ *http.Request) (interface{}, error) {
	fl, err := checkDrivePhotos(s)
	if err != nil {
		return nil, err
	}
	job := newJob(s, "image", len(fl))
	job.files = fl
	addJob(job)
	jobChan <- job
	snap, _ := getJob(job.Id)
	return snap, nil
}

// handleCancelJob stops a queued or running import job (Drive image/video sync or
// a local video upload). The worker
// reacts between files (and, for video, mid-download/transcode); files imported
// before the cancel are kept. A queued job reads CANCELLED immediately; a running one
// turns CANCELLED once the worker has stopped, so its snapshot may still say STARTED.
// Cancelling a job that already ended is a no-op that returns its status.
func (s *mserver) handleCancelJob(r *http.Request) (interface{}, error) {
	id := Var(r, "jobid")
	jobMu.Lock()
	job, found := jobMap[id]
	if found {
		switch job.State {
		case StateScheduled:
			// Not picked up yet (e.g. queued behind a long transcode): report it
			// cancelled now and drop its staged uploads. The worker skips it when it
			// dequeues it and finishes it then (finishJob also clears job.s, which the
			// worker still needs, so it must not run here).
			job.cancel()
			job.State = StateCancelled
			removeStaged(job.videoSources)
		case StateStarted:
			job.cancel()
		}
	}
	jobMu.Unlock()
	if !found {
		return nil, NotFoundError("job not found")
	}
	snap, _ := getJob(id)
	return snap, nil
}

// handleJobStatus returns the status of any import job.
func (s *mserver) handleJobStatus(r *http.Request) (interface{}, error) {
	if job, found := getJob(Var(r, "jobid")); found {
		return job, nil
	} else {
		return nil, NotFoundError("job not found")
	}
}

const StateScheduled = "SCHEDULED"
const StateStarted = "STARTED"
const StateFinished = "FINISHED"
const StateAborted = "ABORTED"     // failed with an error
const StateCancelled = "CANCELLED" // stopped by the owner

// JobFailure records one file that failed within a job (for the UI, e.g.
// "1 skipped: hdr").
type JobFailure struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

type Job struct {
	Id           string        `json:"id"`
	Kind         string        `json:"kind"` // "image" | "video"
	State        string        `json:"state"`
	Percent      int           `json:"percent"`
	files        []*drive.File // image jobs
	videoSources []videoSource // video jobs
	s            *mserver
	NumFiles     int          `json:"numFiles"`
	NumProcessed int          `json:"numProcessed"`
	NumAdded     int          `json:"numAdded"`
	NumSkipped   int          `json:"numSkipped"`
	NumFailed    int          `json:"numFailed"`
	Failures     []JobFailure `json:"failures,omitempty"`
	Err          *ApiError    `json:"error,omitempty"`
	// ctx is cancelled by handleCancelJob; workers check it between files and
	// pass it to downloads/transcodes.
	ctx    context.Context
	cancel context.CancelFunc
}

// newJob returns a SCHEDULED job with its own cancellable context.
func newJob(s *mserver, kind string, numFiles int) *Job {
	ctx, cancel := context.WithCancel(context.Background())
	return &Job{
		Id:       uuid.New().String(),
		Kind:     kind,
		s:        s,
		NumFiles: numFiles,
		State:    StateScheduled,
		ctx:      ctx,
		cancel:   cancel,
	}
}

var jobChan = make(chan *Job, 10)
var wg sync.WaitGroup
var jobMap = make(map[string]*Job)

// jobMu guards jobMap and in-flight job field mutations, which are written by the
// worker goroutines and read by the status handler.
var jobMu sync.Mutex

// jobRetention is how long an ended job stays queryable, so a client polling for
// its result still sees it (a var so tests can shorten it).
var jobRetention = 10 * time.Minute

func addJob(job *Job) {
	jobMu.Lock()
	jobMap[job.Id] = job
	jobMu.Unlock()
}

// getJob returns a snapshot copy of a job, safe to serialize while a worker keeps
// mutating the live job.
func getJob(id string) (*Job, bool) {
	jobMu.Lock()
	defer jobMu.Unlock()
	j, ok := jobMap[id]
	if !ok {
		return nil, false
	}
	cp := *j
	cp.Failures = append([]JobFailure(nil), j.Failures...)
	return &cp, true
}

// jobStart marks a dequeued job as running. Only a SCHEDULED job moves on, so a job
// cancelled while queued keeps reading CANCELLED.
func jobStart(job *Job) {
	jobMu.Lock()
	if job.State == StateScheduled {
		job.State = StateStarted
	}
	jobMu.Unlock()
}

func jobAdded(job *Job)   { jobMu.Lock(); job.NumAdded++; jobMu.Unlock() }
func jobSkipped(job *Job) { jobMu.Lock(); job.NumSkipped++; jobMu.Unlock() }

func jobFailed(job *Job, name, category string) {
	jobMu.Lock()
	job.NumFailed++
	job.Failures = append(job.Failures, JobFailure{Name: name, Category: category})
	jobMu.Unlock()
}

func jobProgress(job *Job) {
	jobMu.Lock()
	job.NumProcessed++
	if job.NumFiles > 0 {
		job.Percent = int(math.Round(float64(job.NumProcessed) / float64(job.NumFiles) * 100))
	}
	jobMu.Unlock()
}

func worker(jobChan <-chan *Job) {
	defer wg.Done()
	for job := range jobChan {
		job.s.l.Infow("Processing job", "jobid", job.Id, "files", job.NumFiles)
		process(job)
	}
}

// process runs an image job. A hard error aborts the whole job (image imports are
// fast and an error usually means a systemic problem); skipped/added are counted.
// A cancel stops it before the next file.
func process(job *Job) {
	if job.ctx.Err() != nil { // cancelled while queued
		finishJob(job, nil)
		return
	}
	jobStart(job)
	for _, f := range job.files {
		if job.ctx.Err() != nil {
			break
		}
		added, err := addDrivePhoto(job.ctx, job.s, f)
		if err != nil {
			finishJob(job, err)
			return
		}
		if added {
			jobAdded(job)
		} else {
			jobSkipped(job)
		}
		jobProgress(job)
	}
	finishJob(job, nil)
}

func finishJob(job *Job, err error) {
	jobMu.Lock()
	defer jobMu.Unlock()
	job.files = nil
	job.videoSources = nil
	job.s = nil
	// A cancel takes precedence: an error caused by the cancel (e.g. an interrupted
	// download) is not a failure.
	cancelled := job.ctx.Err() != nil
	job.cancel() // release the context
	if cancelled {
		job.State = StateCancelled
	} else if err != nil {
		job.State = StateAborted
		job.Err = ResolveError(err)
	} else {
		job.Percent = 100
		job.State = StateFinished
	}
	id := job.Id
	time.AfterFunc(jobRetention, func() {
		jobMu.Lock()
		delete(jobMap, id)
		jobMu.Unlock()
	})
}
