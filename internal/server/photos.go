package server

import (
	"github.com/gorilla/mux"
	"github.com/msvens/mdrive"
	"github.com/msvens/mexif"
	"github.com/msvens/mphotos/internal/model"
	"go.uber.org/zap"
	"google.golang.org/api/drive/v3"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type PhotoFiles struct {
	Length int            `json:"length"`
	Photos []*model.Photo `json:"photos,omitempty"`
}

func deletePhoto(s *mserver, p *model.Photo, removeFiles bool) (*model.Photo, error) {
	s.l.Infow("Delete Photo", "id", p.DriveId, "removeFiles", removeFiles)
	if del, err := s.db.DeletePhoto(p.DriveId); err != nil {
		return nil, err
	} else if !del {
		return nil, NotFoundError("Photo not found")
	}
	if !removeFiles {
		return p, nil
	}
	//remove files
	if err := os.Remove(imgPath(s, p.FileName)); err != nil {
		s.l.Errorw("Could not remove image", zap.Error(err))
	}
	if err := os.Remove(thumbPath(s, p.FileName)); err != nil {
		s.l.Errorw("Could not remove thumbnail", zap.Error(err))
	}
	s.l.Infow("Photo deleted", "id", p.DriveId)
	return p, nil
}

func (s *mserver) handleDeletePhoto(r *http.Request) (interface{}, error) {
	type request struct {
		RemoveFiles bool `json:"removeFiles" schema:"removeFiles"`
	}
	if photo, err := s.db.Photo(Var(r, "id"), true); err != nil {
		return nil, ResolveError(err)
	} else {
		var params request
		if err := decodeRequest(r, &params); err != nil {
			return nil, err
		}
		return deletePhoto(s, photo, params.RemoveFiles)
	}
	//return s.ps.DeletePhoto(photo, params.RemoveFiles)
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
	if photos, err := s.db.Photos(model.Range{}, model.None, model.PhotoFilter{Private: true}); err != nil {
		return nil, err
	} else {
		for _, p := range photos {
			if _, e := deletePhoto(s, p, params.RemoveFiles); e != nil {
				s.l.Errorw("could not delete photo ", "photo", p.DriveId, zap.Error(e))
			}
		}
		return &PhotoFiles{len(photos), photos}, nil
	}
	//return s.ps.DeletePhotos(params.RemoveFiles)
}

func (s *mserver) handleDownloadPhoto(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	loggedIn := s.isLoggedIn(w, r)
	p, err := s.db.Photo(id, loggedIn)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
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
	id := Var(r, "id")
	if !loggedIn && !s.db.HasPhoto(id, false) {
		return nil, NotFoundError("could not find photo")
	}
	if exif, err := s.db.Exif(id); err != nil {
		return nil, err
	} else {
		return exif, nil
	}
}

func (s *mserver) handleImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	http.ServeFile(w, r, imgPath(s, name))
}

func (s *mserver) handleLatestPhoto(_ *http.Request, loggedIn bool) (interface{}, error) {
	photos, err := s.db.Photos(model.Range{Offset: 0, Limit: 1}, model.DriveDate, model.PhotoFilter{Private: loggedIn})
	if err != nil {
		return nil, err
	} else if len(photos) < 1 {
		return nil, NotFoundError("no photos in collection")
	} else {
		return photos[0], nil
	}
}

func (s *mserver) handlePhotoAlbums(r *http.Request, loggedIn bool) (interface{}, error) {
	id := Var(r, "id")
	if !loggedIn && !s.db.HasPhoto(id, loggedIn) {
		return nil, NotFoundError("Could not find photo")
	}
	if albums, err := s.db.PhotoAlbums(id); err != nil {
		return nil, err
	} else {
		names := make([]string, 0)
		for _, a := range albums {
			names = append(names, a.Name)
		}
		return names, nil
	}
}

func (s *mserver) handlePhoto(r *http.Request, loggedIn bool) (interface{}, error) {
	id := Var(r, "id")
	if photo, err := s.db.Photo(id, loggedIn); err != nil {
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
		r := model.Range{Offset: params.Offset, Limit: params.Limit}
		f := model.PhotoFilter{Private: loggedIn}
		if photos, err := s.db.Photos(r, model.DriveDate, f); err != nil {
			return nil, err
		} else {
			return &PhotoFiles{Length: len(photos), Photos: photos}, nil
		}
	}
}

func (s *mserver) handleScheduleJob(_ *http.Request) (interface{}, error) {
	return scheduleAddPhotos(s)
	//return s.ps.ScheduleAddPhotos()
	//return nil, service.InternalError("Schedule Add Photos currently disabled")
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
		f := model.PhotoFilter{loggedIn, params.CameraModel}
		if photos, err := s.db.Photos(model.Range{}, model.DriveDate, f); err != nil {
			return nil, err
		} else {
			return &PhotoFiles{Length: len(photos), Photos: photos}, nil
		}
	} else {
		return nil, InternalError("Search pattern not yet implemented")
	}
}

func (s *mserver) handleStatusJob(r *http.Request) (interface{}, error) {
	//return nil, service.InternalError("Async Jobs currently disabled")
	//return s.ps.JobStatus(Var(r, "id"))
	return jobStatus(s, Var(r, "id"))
}

func (s *mserver) handleThumb(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	http.ServeFile(w, r, thumbPath(s, name))
}

func (s *mserver) handleUpdatePhoto(r *http.Request) (interface{}, error) {
	type request struct {
		Id          string   `json:"id"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Keywords    []string `json:"keywords"`
		Albums      []string `json:"albums"`
	}
	var par request
	if err := decodeRequest(r, &par); err != nil {
		return nil, err
	}
	if photo, err := s.db.UpdatePhoto(par.Title, par.Description, par.Keywords, par.Id); err != nil {
		return nil, err
	} else {
		if err := s.db.UpdatePhotoAlbums(par.Albums, par.Id); err != nil {
			return nil, err
		}
		return photo, err
	}
	//return s.ps.UpdatePhoto(par.Id, par.Title, par.Description, par.Keywords, par.Albums)
}

func (s *mserver) handleUpdatePhotoPrivate(r *http.Request) (interface{}, error) {
	if photo, err := s.db.Photo(Var(r, "id"), true); err != nil {
		return nil, err
	} else {
		return s.db.SetPrivatePhoto(!photo.Private, photo.DriveId)
	}
}

func (s *mserver) handleUpdatePhotos(_ *http.Request) (interface{}, error) {
	return addPhotos(s)
}

func addPhoto(s *mserver, f *drive.File, tool *mexif.MExifTool) (bool, error) {
	var err error
	if s.db.HasPhoto(f.Id, true) {
		return false, nil
	}
	photo := model.Photo{}
	photo.DriveId = f.Id
	//photo.Title = f.Name
	photo.Md5 = f.Md5Checksum
	photo.FileName = f.Id + ".jpg"
	if t, err := mdrive.ParseTime(f.CreatedTime); err == nil {
		photo.DriveDate = t
	}

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
	photo.Likes = 0

	if err = s.db.AddPhoto(&photo, exif); err != nil {
		s.l.Errorw("error adding photo: ", zap.Error(err))
		return false, err
	}
	s.l.Infow("added photo", "driveId", photo.DriveId)
	return true, nil
}

func downloadPhoto(s *mserver, photo *model.Photo) error {

	if _, err := s.ds.Download(photo.DriveId, imgPath(s, photo.FileName)); err != nil {
		return err
	}

	//create thumbnail
	//args := []string{ps.GetImgPath(photo.FileName), "-s", "640", "-m", "centre", "-o", ps.GetThumbPath(photo.FileName)}
	args := []string{imgPath(s, photo.FileName), "-s", "640", "-c", "-o", thumbPath(s, photo.FileName)}
	s.l.Infow("creating thumbnail", "args: ", strings.Join(args, " "))
	cmd := exec.Command("vipsthumbnail", args...)

	if err := cmd.Start(); err != nil {
		s.l.Errorw("could not create thumbnail", zap.Error(err))
		return InternalError(err.Error())
	}

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

func imgPath(s *mserver, fileName string) string {
	return filepath.Join(s.imgDir, fileName)
}

func thumbPath(s *mserver, fileName string) string {
	return filepath.Join(s.thumbDir, fileName)
}
