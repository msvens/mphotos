package server

import (
	"github.com/gorilla/mux"
	"github.com/msvens/mphotos/internal/model"
	"net/http"
	"strconv"
)

type AlbumCollection struct {
	Info   *model.Album `json:"info"`
	Photos *PhotoFiles  `json:"photos"`
}

func (s *mserver) handleAlbums(_ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	return s.db.Albums()
}

/*
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
*/

func (s *mserver) handleAlbum(r *http.Request, loggedIn bool) (interface{}, error) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])
	if album, err := s.db.Album(id); err != nil {
		return nil, err
	} else {
		photos, err := s.db.AlbumPhotos(id, model.PhotoFilter{Private: loggedIn})
		if err != nil {
			return nil, err
		}
		return &AlbumCollection{Info: album, Photos: &PhotoFiles{len(photos), photos}}, nil
	}
}

func (s *mserver) handleDeleteAlbum(r *http.Request) (interface{}, error) {
	id, err := strconv.Atoi(Var(r, "id"))

	album, err := s.db.Album(id)
	if err != nil {
		return nil, err
	}
	if err = s.db.DeleteAlbum(id); err != nil {
		return nil, err
	}
	return album, nil
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

	if s.db.HasAlbumName(param.Name) {
		return nil, BadRequestError("Album name in use")
	}
	return s.db.AddAlbum(param.Name, param.Description, param.CoverPic)
}

func (s *mserver) handlePhotoAlbums(r *http.Request, loggedIn bool) (interface{}, error) {
	id := Var(r, "id")

	if !s.db.HasPhoto(id, loggedIn) { //broken
		return nil, NotFoundError("Could not find photo")
	}
	return s.db.PhotoAlbums(id)
	/*if albums, err := s.db.PhotoAlbums(id); err != nil {
		return nil, err
	} else {
		names := make([]string, 0)
		for _, a := range albums {
			names = append(names, a.Name)
		}
		return names, nil
	}*/
}

func (s *mserver) handleUpdateAlbum(r *http.Request) (interface{}, error) {
	var a model.Album
	if err := decodeRequest(r, &a); err != nil {
		return nil, err
	}
	if s.db.HasAlbum(a.Id) {
		return s.db.UpdateAlbum(&a)
	} else {
		return nil, NotFoundError("Album not found")
	}
}
