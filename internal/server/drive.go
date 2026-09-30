package server

import (
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

func (s *mserver) handleAddDrivePhotos(_ *http.Request) (interface{}, error) {
	return addDrivePhotos(s)
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
// videos are in the Drive folder, plus whether video import is available at all.
type DriveCheck struct {
	Images       int  `json:"images"`
	Videos       int  `json:"videos"`
	VideoEnabled bool `json:"videoEnabled"`
}

func (s *mserver) handleCheckDrive(_ *http.Request) (interface{}, error) {
	images, err := checkDrivePhotos(s)
	if err != nil {
		return nil, err
	}
	check := DriveCheck{Images: len(images), VideoEnabled: s.videoEnabled}
	if s.videoEnabled {
		videos, err := checkDriveVideos(s)
		if err != nil {
			return nil, err
		}
		check.Videos = len(videos)
	}
	return check, nil
}

func addDrivePhoto(s *mserver, f *drive.File) (bool, error) {
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
	if _, err := s.ds.Download(f.Id, stagedPath); err != nil {
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

func addDrivePhotos(s *mserver) (*DriveFiles, error) {
	fl, err := listDriveFiles(s)
	if err != nil {
		return nil, err
	}

	var files []*drive.File
	for _, f := range fl {
		added, err := addDrivePhoto(s, f)
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
	job := &Job{
		Id:       uuid.New().String(),
		Kind:     "image",
		files:    fl,
		s:        s,
		NumFiles: len(fl),
		State:    StateScheduled,
	}
	addJob(job)
	jobChan <- job
	snap, _ := getJob(job.Id)
	return snap, nil
}

func (s *mserver) handleStatusDriveJob(r *http.Request) (interface{}, error) {
	if job, found := getJob(Var(r, "jobid")); found {
		return job, nil
	} else {
		return nil, NotFoundError("job not found")
	}
}

const StateScheduled = "SCHEDULED"
const StateStarted = "STARTED"
const StateFinished = "FINISHED"
const StateAborted = "ABORTED"

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
}

var jobChan = make(chan *Job, 10)
var wg sync.WaitGroup
var jobMap = make(map[string]*Job)

// jobMu guards jobMap and in-flight job field mutations, which are written by the
// worker goroutines and read by the status handler.
var jobMu sync.Mutex

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

func jobSetState(job *Job, state string) {
	jobMu.Lock()
	job.State = state
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
func process(job *Job) {
	jobSetState(job, StateStarted)
	for _, f := range job.files {
		added, err := addDrivePhoto(job.s, f)
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
	if err != nil {
		job.State = StateAborted
		job.Err = ResolveError(err)
	} else {
		job.Percent = 100
		job.State = StateFinished
	}
}
