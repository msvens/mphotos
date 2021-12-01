package server

import (
	"github.com/google/uuid"
	"github.com/msvens/mphotos/internal/dao"
	"net/http"
)

type AlbumCollection struct {
	Info   *dao.Album  `json:"info"`
	Photos *PhotoFiles `json:"photos"`
}

func (s *mserver) handleAlbums(_ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	return s.pg.Album.List()
}

func (s *mserver) handleAlbum(r *http.Request, loggedIn bool) (interface{}, error) {
	if id, err := uuid.Parse(Var(r, "id")); err != nil {
		return nil, BadRequestError("Could not parse album id")
	} else {
		if album, err := s.pg.Album.Get(id); err != nil {
			return nil, err
		} else {
			photos, err := s.pg.Album.Photos(id, loggedIn)
			if err != nil {
				return nil, err
			}
			return &AlbumCollection{Info: album, Photos: &PhotoFiles{len(photos), photos}}, nil
		}
	}
}

func (s *mserver) handleDeleteAlbum(r *http.Request) (interface{}, error) {
	if id, err := uuid.Parse(Var(r, "id")); err != nil {
		return nil, BadRequestError("Could not parse album id")
	} else {
		ret, _ := s.pg.Album.Get(id)
		return ret, s.pg.Album.Delete(id)
	}
}

func (s *mserver) handleAddAlbum(r *http.Request) (interface{}, error) {
	type request struct {
		Name        string
		Description string
		CoverPic    string
	}
	var param request
	if err := decodeRequest(r, &param); err != nil {
		return nil, err
	}
	if s.pg.Album.HasByName(param.Name) {
		return nil, BadRequestError("Album name in use")
	}
	return s.pg.Album.Add(param.Name, param.Description, param.CoverPic)
}

func (s *mserver) handlePhotoAlbums(r *http.Request, loggedIn bool) (interface{}, error) {
	if photoId, err := uuid.Parse(Var(r, "id")); err != nil {
		return nil, BadRequestError("could not parse img id")
	} else {
		if !s.pg.Photo.Has(photoId, loggedIn) {
			return nil, NotFoundError("Could not find img xxx")
		} else {
			return s.pg.Album.Albums(photoId)
		}
	}
}

func (s *mserver) handleUpdateAlbum(r *http.Request) (interface{}, error) {
	var a dao.Album
	if err := decodeRequest(r, &a); err != nil {
		return nil, err
	}
	if s.pg.Album.Has(a.Id) {
		return s.pg.Album.Update(&a)
	} else {
		return nil, NotFoundError("Album not found")
	}
}
