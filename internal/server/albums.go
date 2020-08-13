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
	//return s.db.Ge
	return s.db.Albums()
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
	//return s.ps.GetAlbumCollection(name, loggedIn)
}

//TODO: Handle Delete
func (s *mserver) handleDeleteAlbum(r *http.Request) (interface{}, error) {
	return nil, InternalError("not yet implemented")
	/*vars := mux.Vars(r)
	name := vars["name"]
	collection, err := s.ps.GetAlbumCollection(name, true)
	if err != nil {
		return nil, err
	}
	return collection.Info, nil*/
}

func (s *mserver) handleUpdateAlbum(r *http.Request) (interface{}, error) {
	var a model.Album
	if err := decodeRequest(r, &a); err != nil {
		return nil, err
	}
	//a = model.Album{Name: a.Name, Description: a.Description, CoverPic: a.CoverPic}
	return s.db.UpdateAlbum(&a)
	//return s.ps.UpdateAlbum(a.Description, a.CoverPic, a.Name)
}
