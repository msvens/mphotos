package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/msvens/mimage/metadata"
	"github.com/msvens/mimage/video"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"github.com/msvens/mphotos/internal/gdrive"
	"go.uber.org/zap"
	"google.golang.org/api/drive/v3"
)

// transcodeTimeout bounds a single video transcode so a hung ffmpeg can't wedge
// the worker forever. A long 1080p clip finishes well within this.
const transcodeTimeout = 30 * time.Minute

// videoSource is one video to import, from Drive (needs download) or an
// already-staged local upload.
type videoSource struct {
	id         uuid.UUID
	source     string // dao.SourceGoogle | dao.SourceLocal
	sourceId   string // stored in photo.SourceId
	driveId    string // Drive file id to download; empty for local
	md5        string
	name       string
	sourceDate time.Time
	stagedPath string // pre-staged file (local); empty for Drive until fetched
}

var videoJobChan = make(chan *Job, 10)

// videoWorker processes video jobs one at a time, independently of the image
// worker, so an image sync never waits behind a multi-minute transcode.
func videoWorker(ch <-chan *Job) {
	defer wg.Done()
	for job := range ch {
		job.s.l.Infow("Processing video job", "jobid", job.Id, "files", job.NumFiles)
		job.s.processVideo(job)
	}
}

// processVideo imports each source, continuing past per-file failures (unlike the
// image worker, which aborts): a bad clip (HDR, truncated) is recorded and skipped
// so the rest of the batch still imports.
// A cancel stops it before the next file and interrupts the current one.
func (s *mserver) processVideo(job *Job) {
	if job.ctx.Err() != nil { // cancelled while queued
		removeStaged(job.videoSources)
		finishJob(job, nil)
		return
	}
	jobSetState(job, StateStarted)
	for i, src := range job.videoSources {
		if job.ctx.Err() != nil {
			removeStaged(job.videoSources[i:])
			break
		}
		added, err := s.importVideo(job.ctx, src)
		if err != nil && job.ctx.Err() != nil {
			// interrupted by the cancel, not a bad file: don't record it
			removeStaged(job.videoSources[i+1:])
			break
		}
		if err != nil {
			cat := videoErrorCategory(err)
			if rerr := s.pg.ImportError.Record(&dao.ImportError{
				Md5: src.md5, DriveId: src.driveId, Name: src.name, Category: cat, Message: err.Error(),
			}); rerr != nil {
				s.l.Errorw("could not record import error", zap.Error(rerr))
			}
			jobFailed(job, src.name, cat)
			s.l.Warnw("video import failed", "name", src.name, "category", cat, zap.Error(err))
		} else if added {
			jobAdded(job)
		} else {
			jobSkipped(job)
		}
		jobProgress(job)
	}
	finishJob(job, nil)
}

// removeStaged deletes the pre-staged files (local uploads) of sources that will
// not be imported because the job was cancelled.
func removeStaged(sources []videoSource) {
	for _, src := range sources {
		if src.stagedPath != "" {
			_ = os.Remove(src.stagedPath)
		}
	}
}

// importVideo transcodes one source to the stored <uuid>.mp4, derives the poster
// and its jpeg variants, and persists the row. Returns (added, err): (false, nil)
// means already imported.
func (s *mserver) importVideo(jobCtx context.Context, src videoSource) (bool, error) {
	if s.pg.Photo.HasMd5(src.md5) {
		if src.stagedPath != "" { // local upload staged for nothing
			_ = os.Remove(src.stagedPath)
		}
		return false, nil
	}

	staged := src.stagedPath
	if staged == "" { // Drive: download to a staged temp
		staged = config.PhotoFilePath(config.Original, src.id.String()+".import")
		if _, err := s.ds.Download(jobCtx, src.driveId, staged); err != nil {
			_ = os.Remove(staged)
			return false, err
		}
	}

	ctx, cancel := context.WithTimeout(jobCtx, transcodeTimeout)
	defer cancel()

	base := config.PhotoFilePath(config.Original, src.id.String()) // Original/<uuid>, no ext
	r, err := video.Transcode(ctx, staged, base, video.TranscodeOptions{})
	if err != nil {
		_ = os.Remove(staged)
		return false, err
	}
	_ = os.Remove(staged) // source no longer needed; the stored original is r.Path

	posterBase := config.PhotoFilePath(config.Original, src.id.String()+".poster")
	poster, err := video.ExtractPoster(ctx, r.Path, posterBase, video.PosterOptions{})
	if err != nil {
		_ = os.Remove(r.Path)
		return false, err
	}
	if err := dao.GenerateImagesFromSource(poster, src.id.String()); err != nil {
		_ = os.Remove(poster)
		_ = os.Remove(r.Path)
		return false, err
	}
	_ = os.Remove(poster) // only the variants are kept

	photo := dao.Photo{
		Id:          src.id,
		Kind:        dao.KindVideo,
		Source:      src.source,
		SourceId:    src.sourceId,
		Md5:         src.md5,
		SourceDate:  src.sourceDate,
		UploadDate:  time.Now(),
		FileName:    src.id.String() + ".mp4",
		SourceOther: sourceOtherFor(r.Source),
	}
	applyVideoSummary(&photo, r.Source)
	photo.Width = uint(r.Output.Width)
	photo.Height = uint(r.Output.Height)
	photo.Duration = r.Source.Duration.Seconds()
	photo.OriginalDate = videoOriginalDate(r.Source, src)
	if photo.CameraModel == "" {
		photo.CameraModel = dao.NoCameraModel
	}

	if err := s.pg.Photo.Add(&photo, &metadata.Summary{}); err != nil {
		s.l.Errorw("error adding video", zap.Error(err))
		_ = dao.DeleteImg(photo.FileName) // roll back the stored files
		return false, err
	}
	if !s.pg.Camera.HasModel(photo.CameraModel) {
		if err := s.pg.Camera.AddFromPhoto(&photo); err != nil {
			s.l.Errorw("error adding camera model", zap.Error(err))
			return false, err
		}
	}
	s.l.Infow("added video", "id", photo.Id)
	return true, nil
}

func applyVideoSummary(photo *dao.Photo, sum *video.Summary) {
	photo.CameraMake = sum.CameraMake
	photo.CameraModel = sum.CameraModel
	photo.Title = sum.Title
	photo.Description = sum.Description
	if len(sum.Keywords) > 0 {
		photo.Keywords = strings.Join(sum.Keywords, ",")
	}
}

func sourceOtherFor(sum *video.Summary) string {
	if sum != nil && sum.VideoCodec != "" {
		return sum.VideoCodec
	}
	return dao.KindVideo
}

var dateNameRe = regexp.MustCompile(`(\d{4})(\d{2})(\d{2})[_-](\d{2})(\d{2})(\d{2})`)

// videoOriginalDate resolves the capture date: the probe's creation time if
// present, else a date embedded in the filename (Samsung VID_YYYYMMDD_HHMMSS),
// else the source/upload date, else now.
func videoOriginalDate(sum *video.Summary, src videoSource) time.Time {
	if sum != nil && !sum.CreationTime.IsZero() {
		return sum.CreationTime
	}
	if t, ok := parseDateFromName(src.name); ok {
		return t
	}
	if !src.sourceDate.IsZero() {
		return src.sourceDate
	}
	return time.Now()
}

func parseDateFromName(name string) (time.Time, bool) {
	m := dateNameRe.FindStringSubmatch(name)
	if m == nil {
		return time.Time{}, false
	}
	str := fmt.Sprintf("%s-%s-%s %s:%s:%s", m[1], m[2], m[3], m[4], m[5], m[6])
	t, err := time.ParseInLocation("2006-01-02 15:04:05", str, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// videoErrorCategory maps a transcode/probe error to a short, stable category for
// the import_error record and the job's failure list.
func videoErrorCategory(err error) string {
	switch {
	case errors.Is(err, video.ErrHDRUnsupported):
		return "hdr"
	case errors.Is(err, video.ErrTruncated):
		return "truncated"
	case errors.Is(err, video.ErrNoVideoStream):
		return "no-video-stream"
	case errors.Is(err, video.ErrNotVideo):
		return "not-video"
	}
	var te *video.ToolError
	if errors.As(err, &te) {
		return "tool-error"
	}
	return "error"
}

func listDriveVideoFiles(s *mserver) ([]*drive.File, error) {
	u, err := s.pg.User.Get()
	if err != nil {
		return nil, InternalError("user not found")
	}
	if u.DriveFolderId == "" {
		return nil, NotFoundError("Drive folder has not been set")
	}
	return searchDriveByMime(s, u.DriveFolderId, "", "video/")
}

// checkDriveVideos lists new videos: those not already imported and not previously
// recorded as failed (so a known-bad file isn't re-downloaded/re-transcoded).
func checkDriveVideos(s *mserver) ([]*drive.File, error) {
	fl, err := listDriveVideoFiles(s)
	if err != nil {
		return nil, err
	}
	var ret []*drive.File
	for _, f := range fl {
		if s.pg.Photo.HasMd5(f.Md5Checksum) || s.pg.ImportError.HasMd5(f.Md5Checksum) {
			continue
		}
		ret = append(ret, f)
	}
	return ret, nil
}

func (s *mserver) handleScheduleVideoJob(_ *http.Request) (interface{}, error) {
	if !s.videoEnabled {
		return nil, BadRequestError("video support is not available (ffmpeg missing)")
	}
	fl, err := checkDriveVideos(s)
	if err != nil {
		return nil, err
	}
	sources := make([]videoSource, 0, len(fl))
	for _, f := range fl {
		src := videoSource{
			id:       uuid.New(),
			source:   dao.SourceGoogle,
			sourceId: f.Id,
			driveId:  f.Id,
			md5:      f.Md5Checksum,
			name:     f.Name,
		}
		if t, err := gdrive.ParseTime(f.CreatedTime); err == nil {
			src.sourceDate = t
		}
		sources = append(sources, src)
	}
	job := newJob(s, "video", len(sources))
	job.videoSources = sources
	addJob(job)
	videoJobChan <- job
	snap, _ := getJob(job.Id)
	return snap, nil
}

// uploadLocalVideo stages a locally-uploaded video and enqueues it on the video
// worker, returning a job the client polls (a synchronous transcode would time out
// the request). Called from handleUploadLocalPhoto when the upload is a video.
func (s *mserver) uploadLocalVideo(r *http.Request, file multipart.File, filename, md5str string) (interface{}, error) {
	if !s.videoEnabled {
		return nil, BadRequestError("video support is not available (ffmpeg missing)")
	}
	id := uuid.New()
	staged := config.PhotoFilePath(config.Original, id.String()+".import")
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	dst, err := os.Create(staged)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(dst, file); err != nil {
		_ = dst.Close()
		_ = os.Remove(staged)
		return nil, err
	}
	_ = dst.Close()

	sourceDate := time.Now()
	if v := r.FormValue("sourceDate"); v != "" {
		if d, err := time.Parse(time.RFC3339, v); err == nil {
			sourceDate = d
		}
	}
	sourceId := r.FormValue("sourceId")
	if sourceId == "" {
		sourceId = filename
	}
	src := videoSource{
		id:         id,
		source:     dao.SourceLocal,
		sourceId:   sourceId,
		md5:        md5str,
		name:       filename,
		sourceDate: sourceDate,
		stagedPath: staged,
	}
	job := newJob(s, "video", 1)
	job.videoSources = []videoSource{src}
	addJob(job)
	videoJobChan <- job
	snap, _ := getJob(job.Id)
	return snap, nil
}
