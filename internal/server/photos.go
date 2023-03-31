package server

import (
	"github.com/google/uuid"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"go.uber.org/zap"
	"net/http"
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
	if err := dao.DeleteImg(p.FileName); err != nil {
		s.l.Errorw("Could not remove "+p.FileName, zap.Error(err))
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
				s.l.Errorw("could not delete img ", "img", p.Id, zap.Error(e))
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
	http.ServeFile(w, r, config.PhotoFilePath(config.Original, p.FileName))
	//file, err := os.Open(imgPath(s, p.FileName))
	/*file, err := os.Open(config.PhotoFilePath(config.Original, p.FileName))
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
	return*/
}

func (s *mserver) handleExif(r *http.Request, loggedIn bool) (interface{}, error) {
	id, err := uuid.Parse(Var(r, "id"))
	if err != nil {
		return nil, BadRequestError("Could not parse Id")
	}
	if !loggedIn && !s.pg.Photo.Has(id, false) {
		return nil, NotFoundError("could not find img")
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
		Limit  int
		Offset int
		Order  string
	}

	var params request
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	} else {
		r := dao.Range{Offset: params.Offset, Limit: params.Limit}
		f := dao.PhotoFilter{Private: loggedIn}
		var o dao.PhotoOrder
		switch params.Order {
		case "original":
			o = dao.OriginalDate
		default:
			o = dao.UploadDate

		}
		if photos, err := s.pg.Photo.Select(r, o, f); err != nil {
			return nil, err
		} else {
			return &PhotoFiles{Length: len(photos), Photos: photos}, nil
		}
	}
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
