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

	id, err := uuid.Parse(Var(r, "photoid"))
	if err != nil {
		return nil, BadRequestError("Could not parse Id")
	}
	if photo, err := s.pg.Photo.Get(id); err != nil {
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
	id, err := uuid.Parse(Var(r, "photoid"))
	if err != nil {
		http.Error(w, "Could not parse Id", http.StatusBadRequest)
		return
	}
	p, err := s.pg.Photo.Get(id)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, config.PhotoFilePath(config.Original, p.FileName))
}

func (s *mserver) handlePhotoAlbums(r *http.Request, loggedIn bool) (interface{}, error) {
	id, err := uuid.Parse(Var(r, "photoid"))
	if err != nil {
		return nil, BadRequestError("could not parse img id")
	}
	albums, err := s.pg.Photo.Albums(id)
	if err != nil {
		return nil, err
	}
	if !loggedIn {
		for i := range albums {
			albums[i].Code = ""
		}
	}
	return albums, nil
}

func (s *mserver) handleAddPhotoAlbums(r *http.Request) (interface{}, error) {
	type request struct {
		AlbumIds []uuid.UUID
	}
	id, err := uuid.Parse(Var(r, "photoid"))
	if err != nil {
		return nil, BadRequestError("Could not parse album id")
	}
	var param request
	if err = decodeRequest(r, &param); err != nil {
		return nil, err
	}
	rows, err := s.pg.Photo.AddAlbums(id, param.AlbumIds)
	if err != nil {
		return nil, err
	}

	return AffectedItems{NumItems: rows}, err
}

func (s *mserver) handleClearPhotoAlbums(r *http.Request) (interface{}, error) {
	id, err := uuid.Parse(Var(r, "photoid"))
	if err != nil {
		return nil, BadRequestError("Could not parse album id")
	}
	rows, err := s.pg.Photo.ClearAlbums(id)
	if err != nil {
		return nil, err
	}
	return AffectedItems{NumItems: rows}, err
}

func (s *mserver) handleDeletePhotoAlbums(r *http.Request) (interface{}, error) {
	type request struct {
		AlbumIds []uuid.UUID
	}
	id, err := uuid.Parse(Var(r, "photoid"))
	if err != nil {
		return nil, BadRequestError("Could not parse album id")
	}
	var param request
	if err = decodeRequest(r, &param); err != nil {
		return nil, err
	}
	rows, err := s.pg.Photo.DeleteAlbums(id, param.AlbumIds)
	if err != nil {
		return nil, err
	}

	return AffectedItems{NumItems: rows}, err
}

func (s *mserver) handleSetPhotoAlbums(r *http.Request) (interface{}, error) {
	type request struct {
		AlbumIds []uuid.UUID
	}
	id, err := uuid.Parse(Var(r, "photoid"))
	if err != nil {
		return nil, BadRequestError("Could not parse album id")
	}
	var param request
	if err = decodeRequest(r, &param); err != nil {
		return nil, err
	}
	rows, err := s.pg.Photo.SetAlbums(id, param.AlbumIds)
	if err != nil {
		return nil, err
	}
	return AffectedItems{NumItems: rows}, err
}

func (s *mserver) handleExif(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	id, err := uuid.Parse(Var(r, "photoid"))
	if err != nil {
		return nil, BadRequestError("Could not parse Id")
	}
	if !s.pg.Photo.Has(id) {
		return nil, NotFoundError("could not find img")
	}
	if exif, err := s.pg.Photo.Exif(id); err != nil {
		return nil, err
	} else {
		return exif, nil
	}
}

func (s *mserver) handlePhoto(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	id, err := uuid.Parse(Var(r, "photoid"))
	if err != nil {
		return nil, BadRequestError("Could not parse Id")
	}
	if photo, err := s.pg.Photo.Get(id); err != nil {
		return nil, err
	} else {
		return photo, nil
	}
}

// handlePhotos lists photos with optional paging and ordering. It is public
// (loginInfo): a logged-in owner gets every photo, while a guest gets only the
// members of the photostream album. That album is a typed user setting; if it is
// unset, guests get an empty list.
func (s *mserver) handlePhotos(r *http.Request, loggedIn bool) (interface{}, error) {
	type request struct {
		Limit   int
		Offset  int
		OrderBy dao.PhotoOrder
	}
	var params request
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}

	order := params.OrderBy
	if order == dao.None {
		order = dao.UploadDate
	}

	var filter dao.PhotoFilter
	if !loggedIn {
		user, err := s.pg.User.Get()
		if err != nil {
			return nil, err
		}
		if user.PhotoStreamAlbumId == "" {
			return &PhotoFiles{Length: 0, Photos: []*dao.Photo{}}, nil
		}
		albumId, err := uuid.Parse(user.PhotoStreamAlbumId)
		if err != nil {
			return nil, InternalError("stored photostream album id is not a valid uuid")
		}
		filter.AlbumId = &albumId
	}

	photos, err := s.pg.Photo.Select(filter, dao.Range{Offset: params.Offset, Limit: params.Limit}, order)
	if err != nil {
		return nil, err
	}
	return &PhotoFiles{Length: len(photos), Photos: photos}, nil
}

// handleUpdatePhoto updates the photo named by {photoid}. The path identifies the photo;
// any id in the request body is ignored.
func (s *mserver) handleUpdatePhoto(r *http.Request) (interface{}, error) {
	type request struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Keywords    []string `json:"keywords"`
	}
	var par request
	var id uuid.UUID
	if err := decodeRequest(r, &par); err != nil {
		return nil, err
	}
	if err := uid(r, "photoid", &id); err != nil {
		return nil, err
	}
	return s.pg.Photo.Set(par.Title, par.Description, par.Keywords, id)
}
