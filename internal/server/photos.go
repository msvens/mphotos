package server

import (
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/msvens/mexif"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"github.com/msvens/mphotos/internal/gdrive"
	"github.com/msvens/mphotos/internal/img"
	"go.uber.org/zap"
	"google.golang.org/api/drive/v3"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PhotoFiles struct {
	Length int          `json:"length"`
	Photos []*dao.Photo `json:"photos,omitempty"`
}

func deletePhoto(s *mserver, p *dao.Photo, removeFiles bool) (*dao.Photo, error) {
	s.l.Infow("Delete Photo", "id", p.Id, "removeFiles", removeFiles)
	if del, err := s.pg.Photo.Delete(p.Id); err != nil {
		return nil, err
	} else if !del {
		return nil, NotFoundError("Photo not found")
	}
	if !removeFiles {
		return p, nil
	}
	//remove files
	if err := os.Remove(imgPath(s, p.FileName)); err != nil {
		s.l.Errorw("Could not remove img", zap.Error(err))
	}
	if err := os.Remove(thumbPath(s, p.FileName)); err != nil {
		s.l.Errorw("Could not remove thumbnail", zap.Error(err))
	}
	s.l.Infow("Photo deleted", "id", p.Id)
	return p, nil
}

func (s *mserver) handleDeletePhoto(r *http.Request) (interface{}, error) {
	type request struct {
		RemoveFiles bool `json:"removeFiles" schema:"removeFiles"`
	}

	id, err := uuid.Parse(Var(r, "id"))
	if err != nil {
		return nil, BadRequestError("Could not parse Id")
	}
	if photo, err := s.pg.Photo.Get(id, true); err != nil {
		return nil, err
	} else {
		var params request
		if err := decodeRequest(r, &params); err != nil {
			return nil, err
		}
		return deletePhoto(s, photo, params.RemoveFiles)
	}
}

func (s *mserver) handleDeletePhotos(r *http.Request) (interface{}, error) {
	type request struct {
		RemoveFiles bool `json:"removeFiles" schema:"removeFiles"`
	}
	var params request
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	s.l.Infow("Delete All Photos", "removeFiles", params.RemoveFiles)
	if photos, err := s.pg.Photo.List(); err != nil {
		return nil, err
	} else {
		for _, p := range photos {
			if _, e := deletePhoto(s, p, params.RemoveFiles); e != nil {
				s.l.Errorw("could not delete photo ", "photo", p.Id, zap.Error(e))
			}
		}
		return &PhotoFiles{len(photos), photos}, nil
	}
}

func (s *mserver) handleDownloadPhoto(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(Var(r, "id"))
	if err != nil {
		http.Error(w, "Could not parse Id", http.StatusBadRequest)
		return
	}
	loggedIn := ctxLoggedIn(r.Context())
	p, err := s.pg.Photo.Get(id, loggedIn)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	file, err := os.Open(imgPath(s, p.FileName))
	if err != nil {
		s.l.Infow("could not download file", zap.Error(err))
		http.Error(w, "File not found.", http.StatusNotFound)
		return
	}
	defer file.Close() //Close after function return
	FileHeader := make([]byte, 512)

	//Copy the headers into the FileHeader buffer
	file.Read(FileHeader)

	//Get content type of file
	FileContentType := http.DetectContentType(FileHeader)

	//Get the file size
	FileStat, _ := file.Stat()                         //Get info from file
	FileSize := strconv.FormatInt(FileStat.Size(), 10) //Get file size as a string

	//Send the headers
	//w.Header().Set("Content-Disposition", "attachment; filename="+path.Base(p.Path))
	w.Header().Set("Content-Type", FileContentType)
	w.Header().Set("Content-Length", FileSize)

	//Send the file
	//We read 512 bytes from the file already, so we reset the offset back to 0
	file.Seek(0, 0)
	io.Copy(w, file) //'Copy' the file to the client
	return
}

func (s *mserver) handleExif(r *http.Request, loggedIn bool) (interface{}, error) {
	id, err := uuid.Parse(Var(r, "id"))
	if err != nil {
		return nil, BadRequestError("Could not parse Id")
	}
	if !loggedIn && !s.pg.Photo.Has(id, false) {
		return nil, NotFoundError("could not find photo")
	}
	if exif, err := s.pg.Photo.Exif(id); err != nil {
		return nil, err
	} else {
		return exif, nil
	}
}

func (s *mserver) handleLatestPhoto(_ *http.Request, loggedIn bool) (interface{}, error) {
	photos, err := s.pg.Photo.Select(dao.Range{Offset: 0, Limit: 1}, dao.UploadDate, dao.PhotoFilter{Private: loggedIn})
	if err != nil {
		return nil, err
	} else if len(photos) < 1 {
		return nil, NotFoundError("no photos in collection")
	} else {
		return photos[0], nil
	}
}

func (s *mserver) handlePhoto(r *http.Request, loggedIn bool) (interface{}, error) {
	id, err := uuid.Parse(Var(r, "id"))
	if err != nil {
		return nil, BadRequestError("Could not parse Id")
	}
	if photo, err := s.pg.Photo.Get(id, loggedIn); err != nil {
		return nil, err
	} else {
		return photo, nil
	}
}

func (s *mserver) handlePhotos(r *http.Request, loggedIn bool) (interface{}, error) {
	type request struct {
		Limit        int
		Offset       int
		OriginalDate bool
	}

	var params request
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	} else {
		r := dao.Range{Offset: params.Offset, Limit: params.Limit}
		f := dao.PhotoFilter{Private: loggedIn}
		if photos, err := s.pg.Photo.Select(r, dao.UploadDate, f); err != nil {
			return nil, err
		} else {
			return &PhotoFiles{Length: len(photos), Photos: photos}, nil
		}
	}
}

func (s *mserver) handleScheduleJob(_ *http.Request) (interface{}, error) {
	fl, err := checkPhotosDrive(s)
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

func (s *mserver) handleSearchPhotos(r *http.Request, loggedIn bool) (interface{}, error) {
	type request struct {
		CameraModel string
		FocalLength string
		Title       string
		Keywords    string
		Description string
		Generic     string
	}

	var params request
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	if params.CameraModel != "" {
		f := dao.PhotoFilter{loggedIn, params.CameraModel}
		if photos, err := s.pg.Photo.Select(dao.Range{}, dao.UploadDate, f); err != nil {
			return nil, err
		} else {
			return &PhotoFiles{Length: len(photos), Photos: photos}, nil
		}
	} else {
		return nil, InternalError("Search pattern not yet implemented")
	}
}

func (s *mserver) handleStatusJob(r *http.Request) (interface{}, error) {
	if job, found := jobMap[Var(r, "id")]; found {
		return job, nil
	} else {
		return nil, NotFoundError("job not found")
	}
}

func (s *mserver) handleImg(dir string, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	http.ServeFile(w, r, filepath.Join(dir, name))
}

func (s *mserver) handleImage(w http.ResponseWriter, r *http.Request) {
	s.handleImg(s.imgDir, w, r)
}

func (s *mserver) handleResize(w http.ResponseWriter, r *http.Request) {
	s.handleImg(s.resizeDir, w, r)
}

func (s *mserver) handlePortrait(w http.ResponseWriter, r *http.Request) {
	s.handleImg(s.portraitDir, w, r)
}

func (s *mserver) handleLandscape(w http.ResponseWriter, r *http.Request) {
	s.handleImg(s.landscapeDir, w, r)
}

func (s *mserver) handleSquare(w http.ResponseWriter, r *http.Request) {
	s.handleImg(s.squareDir, w, r)
}

func (s *mserver) handleThumb(w http.ResponseWriter, r *http.Request) {
	s.handleImg(s.thumbDir, w, r)
}

func (s *mserver) handleUpdatePhoto(r *http.Request) (interface{}, error) {
	type request struct {
		Id          uuid.UUID   `json:"id"`
		Title       string      `json:"title"`
		Description string      `json:"description"`
		Keywords    []string    `json:"keywords"`
		Albums      []uuid.UUID `json:"albums"`
	}
	var par request
	if err := decodeRequest(r, &par); err != nil {
		return nil, err
	}
	if photo, err := s.pg.Photo.Set(par.Title, par.Description, par.Keywords, par.Id); err != nil {
		return nil, err
	} else {
		if err := s.pg.Album.UpdatePhoto(par.Albums, par.Id); err != nil {
			return nil, err
		}
		return photo, err
	}
}

func (s *mserver) handleUpdatePhotoPrivate(r *http.Request) (interface{}, error) {
	id, err := uuid.Parse(Var(r, "id"))
	if err != nil {
		return nil, BadRequestError("Could not parse Id")
	}

	if photo, err := s.pg.Photo.Get(id, true); err != nil {
		return nil, err
	} else {
		return s.pg.Photo.SetPrivate(!photo.Private, photo.Id)
	}
}

func (s *mserver) handleUpdatePhotos(_ *http.Request) (interface{}, error) {
	return addPhotos(s)
}

func addPhoto(s *mserver, f *drive.File, tool *mexif.MExifTool) (bool, error) {
	var err error
	if s.pg.Photo.HasMd5(f.Md5Checksum) {
		return false, nil
	}
	photo := dao.Photo{}
	photo.Id = uuid.New()
	photo.SourceId = f.Id
	photo.Md5 = f.Md5Checksum
	photo.FileName = f.Id + ".jpg"
	if t, err := gdrive.ParseTime(f.CreatedTime); err == nil {
		photo.SourceDate = t
	}
	photo.UploadDate = time.Now()

	if err = downloadPhoto(s, &photo); err != nil {
		s.l.Errorw("error downloading photo", zap.Error(err))
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
		photo.OriginalDate = exif.OriginalDate
	} else {
		return false, err
	}
	photo.Private = true

	if err = s.pg.Photo.Add(&photo, exif); err != nil {
		s.l.Errorw("error adding photo: ", zap.Error(err))
		return false, err
	}
	if !s.pg.Camera.HasModel(photo.CameraModel) {
		if err = s.pg.Camera.AddFromPhoto(&photo); err != nil {
			s.l.Fatalw("error adding camera model: ", zap.Error(err))
		}
	}
	s.l.Infow("added photo", "driveId", photo.Id)
	return true, nil
}

func downloadPhoto(s *mserver, photo *dao.Photo) error {

	if _, err := s.ds.Download(photo.SourceId, imgPath(s, photo.FileName)); err != nil {
		return err
	}

	//create photo versions
	return img.GenerateImages(imgPath(s, photo.FileName), config.ServiceRoot())
	return nil
}

func addPhotos(s *mserver) (*DriveFiles, error) {
	fl, err := listDriveFiles(s)
	if err != nil {
		return nil, err
	}

	tool, err := mexif.NewMExifTool()
	if err != nil {
		return nil, err
	}

	defer tool.Close()

	var files []*drive.File
	for _, f := range fl {
		added, err := addPhoto(s, f, tool)
		if err != nil {
			return nil, err
		}
		if added {
			files = append(files, f)
		}
	}
	return toDriveFiles(files), nil
}

func cameraPath(s *mserver, fileName string) string {
	return filepath.Join(s.cameraDir, fileName)
}

func imgPath(s *mserver, fileName string) string {
	return filepath.Join(s.imgDir, fileName)
}

func thumbPath(s *mserver, fileName string) string {
	return filepath.Join(s.thumbDir, fileName)
}

//async
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

	tool, err := mexif.NewMExifTool()
	defer tool.Close()

	if err != nil {
		finishJob(job, err)
		return
	}

	job.State = StateStarted

	for _, f := range job.files {
		if _, err := addPhoto(job.s, f, tool); err != nil {
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
