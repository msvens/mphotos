package server

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/msvens/mimage/metadata"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"github.com/msvens/mphotos/internal/gdrive"
	"go.uber.org/zap"
	"google.golang.org/api/drive/v3"
	"math"
	"net/http"
	"strings"
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

func (s *mserver) handleCheckDrive(_ *http.Request) (interface{}, error) {
	if files, err := checkDrivePhotos(s); err != nil {
		fmt.Println("in checkDrivePhotos failed: ", err)
		return nil, err
	} else {
		fmt.Println("in toDriveFiles")
		return toDriveFiles(files), nil
	}
}

func addDrivePhoto(s *mserver, f *drive.File) (bool, error) {
	var err error
	if s.pg.Photo.HasMd5(f.Md5Checksum) {
		return false, nil
	}
	photo := dao.Photo{}
	photo.Id = uuid.New()
	photo.SourceId = f.Id
	photo.Md5 = f.Md5Checksum
	photo.Source = dao.SourceGoogle

	//photo.FileName = f.Id + ".jpg"
	photo.FileName = photo.Id.String() + ".jpg" //use the same filename naming convention for gdrive and local files
	if t, err := gdrive.ParseTime(f.CreatedTime); err == nil {
		photo.SourceDate = t
	}
	photo.UploadDate = time.Now()

	if err = downloadDrivePhoto(s, &photo); err != nil {
		s.l.Errorw("error downloading img", zap.Error(err))
		return false, err
	}
	var md *metadata.MetaData

	if md, err = metadata.ParseFile(config.PhotoFilePath(config.Original, photo.FileName)); err == nil {
		photo.CameraMake = md.Summary.CameraMake
		photo.CameraModel = md.Summary.CameraModel
		photo.FocalLength = fmt.Sprintf("%v mm", md.Summary.FocalLength.Float32())
		photo.FocalLength35 = fmt.Sprintf("%v mm", md.Summary.FocalLengthIn35mmFormat)
		photo.LensMake = md.Summary.LensMake
		photo.LensModel = md.Summary.LensModel
		photo.Exposure = md.Summary.ExposureTime.String()
		photo.Width = md.ImageWidth
		photo.Height = md.ImageHeight
		photo.FNumber = md.Summary.FNumber.Float32()
		photo.Iso = uint(md.Summary.ISO)
		photo.Title = md.Summary.Title
		if len(md.Summary.Keywords) > 0 {
			photo.Keywords = strings.Join(md.Summary.Keywords, ",")
		}
		if md.Summary.OriginalDate.IsZero() {
			photo.OriginalDate = photo.SourceDate
		} else {
			photo.OriginalDate = md.Summary.OriginalDate
		}
	} else {
		return false, err
	}
	photo.Private = true

	if err = s.pg.Photo.Add(&photo, md.Summary); err != nil {
		s.l.Errorw("error adding img: ", zap.Error(err))
		return false, err
	}
	if !s.pg.Camera.HasModel(photo.CameraModel) {
		if err = s.pg.Camera.AddFromPhoto(&photo); err != nil {
			s.l.Fatalw("error adding camera model: ", zap.Error(err))
		}
	}
	s.l.Infow("added img", "driveId", photo.Id)
	return true, nil
}

/*
func addDrivePhoto(s *mserver, f *drive.File, tool *mexif.MExifTool) (bool, error) {
	var err error
	if s.pg.Photo.HasMd5(f.Md5Checksum) {
		return false, nil
	}
	photo := dao.Photo{}
	photo.Id = uuid.New()
	photo.SourceId = f.Id
	photo.Md5 = f.Md5Checksum
	photo.Source = dao.SourceGoogle

	photo.FileName = f.Id + ".jpg" //this needs to change to actually check the filename
	if t, err := gdrive.ParseTime(f.CreatedTime); err == nil {
		photo.SourceDate = t
	}
	photo.UploadDate = time.Now()

	if err = downloadDrivePhoto(s, &photo); err != nil {
		s.l.Errorw("error downloading img", zap.Error(err))
		return false, err
	}
	var exif *mexif.ExifCompact

	if exif, err = tool.ExifCompact(imgPath(s, photo.FileName)); err == nil {
		photo.CameraMake = exif.CameraMake
		photo.CameraModel = exif.CameraModel
		photo.FocalLength = exif.FocalLength
		photo.FocalLength35 = exif.FocalLengthIn35mmFormat
		photo.LensMake = exif.LensMake
		photo.LensModel = exif.LensModel
		photo.Exposure = exif.ExposureTime
		photo.Width = exif.ImageWidth
		photo.Height = exif.ImageHeight
		photo.FNumber = exif.FNumber
		photo.Iso = exif.ISO
		photo.Title = exif.Title
		if len(exif.Keywords) > 0 {
			photo.Keywords = strings.Join(exif.Keywords, ",")
		}
		if exif.OriginalDate.IsZero() {
			photo.OriginalDate = photo.SourceDate
		} else {
			photo.OriginalDate = exif.OriginalDate
		}
	} else {
		return false, err
	}
	photo.Private = true

	if err = s.pg.Photo.Add(&photo, exif); err != nil {
		s.l.Errorw("error adding img: ", zap.Error(err))
		return false, err
	}
	if !s.pg.Camera.HasModel(photo.CameraModel) {
		if err = s.pg.Camera.AddFromPhoto(&photo); err != nil {
			s.l.Fatalw("error adding camera model: ", zap.Error(err))
		}
	}
	s.l.Infow("added img", "driveId", photo.Id)
	return true, nil
}
*/

func addDrivePhotos(s *mserver) (*DriveFiles, error) {
	fl, err := listDriveFiles(s)
	if err != nil {
		return nil, err
	}

	/*
		tool, err := mexif.NewMExifTool()
		if err != nil {
			return nil, err
		}

		defer tool.Close()
	*/
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

func downloadDrivePhoto(s *mserver, photo *dao.Photo) error {

	if _, err := s.ds.Download(photo.SourceId, config.PhotoFilePath(config.Original, photo.FileName)); err != nil {
		return err
	}

	//create img versions
	//return GenerateImages(config.PhotoFilePath(config.Original, photo.FileName), config.ServiceRoot())
	return dao.GenerateImages(photo.FileName)
}

func checkDrivePhotos(s *mserver) ([]*drive.File, error) {
	fl, err := listDriveFiles(s)
	fmt.Println("list drive files: ", len(fl), "error: ", err)
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
	fmt.Println("in listDriveFiles")
	if u, err := s.pg.User.Get(); err != nil {
		return nil, InternalError("user not found")
	} else if u.DriveFolderId == "" {
		return nil, NotFoundError("Drive folder has not been set")
	} else {
		return searchDriveFiles(s, u.DriveFolderId, "")
	}
}

func searchDriveFiles(s *mserver, id string, name string) ([]*drive.File, error) {
	fmt.Println("in searchDriveFiles")
	if name != "" {
		fmt.Println("search files: ", name)
		if f, err := s.ds.GetByName(name, true, false, fileFields); err != nil {
			return nil, err
		} else {
			id = f.Id
		}
	}
	fmt.Println("Finding folder: ", id)
	query := gdrive.NewQuery().Parents().In(id).And().MimeType().Eq(gdrive.Jpeg).TrashedEq(false)
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

//async
func (s *mserver) handleScheduleDriveJob(_ *http.Request) (interface{}, error) {
	fl, err := checkDrivePhotos(s)
	if err != nil {
		return nil, err
	}
	job := Job{}
	job.Id = uuid.New().String()
	job.files = fl
	job.s = s
	job.NumFiles = len(fl)
	job.State = StateScheduled
	jobMap[job.Id] = &job
	jobChan <- &job
	return &job, nil
}

func (s *mserver) handleStatusDriveJob(r *http.Request) (interface{}, error) {
	if job, found := jobMap[Var(r, "id")]; found {
		return job, nil
	} else {
		return nil, NotFoundError("job not found")
	}
}

const StateScheduled = "SCHEDULED"
const StateStarted = "STARTED"
const StateFinished = "FINISHED"
const StateAborted = "ABORTED"

type Job struct {
	Id           string `json:"id"`
	State        string `json:"state"`
	Percent      int    `json:"percent"`
	files        []*drive.File
	s            *mserver
	NumFiles     int       `json:"numFiles"`
	NumProcessed int       `json:"numProcessed"`
	Err          *ApiError `json:"error,omitempty"`
}

var jobChan = make(chan *Job, 10)
var wg sync.WaitGroup
var jobMap = make(map[string]*Job)

func worker(jobChan <-chan *Job) {

	defer wg.Done()

	for job := range jobChan {
		job.s.l.Infow("Processing job", "jobid", job.Id, "files", job.NumFiles)
		process(job)
	}
}

func process(job *Job) {

	/*
		tool, err := mexif.NewMExifTool()
		defer tool.Close()

		if err != nil {
			finishJob(job, err)
			return
		}
	*/

	job.State = StateStarted

	for _, f := range job.files {
		if _, err := addDrivePhoto(job.s, f); err != nil {
			finishJob(job, err)
			return
		}
		job.NumProcessed = job.NumProcessed + 1
		percent := float64(job.NumProcessed) / float64(job.NumFiles)
		job.Percent = int(math.Round(percent * 100))
		//fmt.Println(job.Percent, job.NumFiles, job.NumProcessed)
		job.s.l.Debugw("", "jobid", job.Id, "progress", job.Percent)
	}
	finishJob(job, nil)
}

func finishJob(job *Job, err error) {
	job.files = nil
	job.s = nil
	if err != nil {
		job.State = StateAborted
		job.Err = ResolveError(err)
	} else {
		job.Percent = 100
		job.State = StateFinished
	}
}
