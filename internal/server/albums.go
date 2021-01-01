package server

import (
	"github.com/gorilla/mux"
	"github.com/msvens/mphotos/internal/model"
	"net/http"
)

type AlbumCollection struct {
	Info   *model.Album `json:"info"`
	Photos *PhotoFiles  `json:"photos"`
}

func (s *mserver) handleAlbums(_ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	return s.db.Albums()
}

func (s *mserver) handleAlbumCameras(_ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	return s.db.CameraAlbums()
}

func (s *mserver) handleAlbumCamera(r *http.Request, loggedIn bool) (interface{}, error) {
	vars := mux.Vars(r)
	name := vars["name"]
	if album, err := s.db.CameraAlbum(name); err != nil {
		return nil, err
	} else {
		photos, err := s.db.Photos(model.Range{}, model.DriveDate, model.PhotoFilter{loggedIn, name})
		if err != nil {
			return nil, err
		}
		return &AlbumCollection{album, &PhotoFiles{len(photos), photos}}, nil
	}

}

func (s *mserver) handleAlbum(r *http.Request, loggedIn bool) (interface{}, error) {
	vars := mux.Vars(r)
	name := vars["name"]
	if album, err := s.db.Album(name); err != nil {
		return nil, err
	} else {
		photos, err := s.db.AlbumPhotos(name, model.PhotoFilter{Private: loggedIn})
		if err != nil {
			return nil, err
		}
		return &AlbumCollection{Info: album, Photos: &PhotoFiles{len(photos), photos}}, nil
	}
}

func (s *mserver) handleDeleteAlbum(r *http.Request) (interface{}, error) {
	name := Var(r, "name")

	album, err := s.db.Album(name)
	if err != nil {
		return nil, err
	}
	if err = s.db.DeleteAlbum(name); err != nil {
		return nil, err
	}
	return album, nil

}

func (s *mserver) handleUpdateAlbum(r *http.Request) (interface{}, error) {
	var a model.Album
	if err := decodeRequest(r, &a); err != nil {
		return nil, err
	}
	if s.db.HasAlbum(a.Name) {
		return s.db.UpdateAlbum(&a)
	} else {
		s.l.Debugw("Album not found add", "Album", a.Name)
		return &a, s.db.AddAlbum(&a)
	}

}
