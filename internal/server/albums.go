package server

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/msvens/mphotos/internal/dao"
	"net/http"
)

/*
type AlbumCollection struct {
	Info   *dao.Album  `json:"info"`
	Photos *PhotoFiles `json:"photos"`
}*/

type AffectedItems struct {
	NumItems int `json:"numItems"`
}

// TODO: Add login info to remove codes if not logged in
func (s *mserver) handleAlbums(_ *http.Request, loggedIn bool) (interface{}, error) {
	albums, err := s.pg.Album.List()
	if err != nil {
		return nil, err
	}
	//Todo: change to remove the album all together
	if !loggedIn {
		ret := make([]*dao.Album, 0)
		for _, a := range albums {
			if a.Code == "" {
				ret = append(ret, a)
			}
		}
		return ret, nil
	}
	return albums, nil
}

func (s *mserver) handleAlbumByName(r *http.Request, loggedIn bool) (interface{}, error) {
	name := Var(r, "name")
	if name == "" {
		return nil, BadRequestError("empty album name")
	}
	if a, err := s.pg.Album.GetByName(name); err != nil {
		return nil, err
	} else if loggedIn {
		return a, nil
	} else {
		a.Code = ""
		return a, nil
	}
}

func (s *mserver) handleAlbum(r *http.Request, loggedIn bool) (interface{}, error) {
	id, err := uuid.Parse(Var(r, "albumid"))
	if err != nil {
		return nil, BadRequestError("Could not parse album id")
	}
	a, err := s.pg.Album.Get(id)
	if err != nil {
		return nil, err
	} else if loggedIn {
		return a, nil
	} else {
		a.Code = ""
		return a, nil
	}
}

// TODO: fix to add photo filter
func (s *mserver) handleAlbumPhotos(_ http.ResponseWriter, r *http.Request) (interface{}, error) {
	type request struct {
		Code        string
		CameraModel string
		Offset      int
		Limit       int
		OrderBy     dao.PhotoOrder
	}
	id, err := uuid.Parse(Var(r, "albumid"))
	if err != nil {
		return nil, BadRequestError("Could not parse album id")
	}
	//get album:
	album, err := s.pg.Album.Get(id)
	if err != nil {
		return nil, err
	}
	var param request
	if err := decodeRequest(r, &param); err != nil {
		return nil, BadRequestError("Could not parse Album Code")
	} else if album.Code != param.Code {
		return nil, UnauthorizedError("Album code did not match")
	}
	filter := dao.PhotoFilter{CameraModel: param.CameraModel}
	page := dao.Range{Offset: param.Offset, Limit: param.Limit}
	order := album.OrderBy
	if param.OrderBy != dao.None {
		order = param.OrderBy
	}
	if photos, err := s.pg.Album.SelectPhotos(album.Id, filter, page, order); err != nil {
		return nil, err
	} else {
		return PhotoFiles{Photos: photos, Length: len(photos)}, nil
	}

}

func (s *mserver) handleDeleteAlbum(r *http.Request) (interface{}, error) {
	if id, err := uuid.Parse(Var(r, "albumid")); err != nil {
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

func (s *mserver) handleAddAlbumPhotos(r *http.Request) (interface{}, error) {
	type request struct {
		PhotoIds []uuid.UUID
	}
	id, err := uuid.Parse(Var(r, "albumid"))
	if err != nil {
		return nil, BadRequestError("Could not parse album id")
	}
	var param request
	if err = decodeRequest(r, &param); err != nil {
		return nil, err
	}
	rows, err := s.pg.Album.AddPhotos(id, param.PhotoIds)
	if err != nil {
		return nil, err
	}

	return AffectedItems{NumItems: rows}, err
}

func (s *mserver) handleClearAlbumPhotos(r *http.Request) (interface{}, error) {
	id, err := uuid.Parse(Var(r, "albumid"))
	if err != nil {
		return nil, BadRequestError("Could not parse album id")
	}
	rows, err := s.pg.Album.ClearPhotos(id)
	if err != nil {
		return nil, err
	}
	return AffectedItems{NumItems: rows}, err
}

func (s *mserver) handleDeleteAlbumPhotos(r *http.Request) (interface{}, error) {
	type request struct {
		PhotoIds []uuid.UUID
	}
	id, err := uuid.Parse(Var(r, "albumid"))
	if err != nil {
		return nil, BadRequestError("Could not parse album id")
	}
	var param request
	if err = decodeRequest(r, &param); err != nil {
		return nil, err
	}
	rows, err := s.pg.Album.DeletePhotos(id, param.PhotoIds)
	if err != nil {
		return nil, err
	}
	return AffectedItems{NumItems: rows}, err
}

func (s *mserver) handleSetAlbumPhotos(r *http.Request) (interface{}, error) {
	type request struct {
		PhotoIds []uuid.UUID
	}
	id, err := uuid.Parse(Var(r, "albumid"))
	if err != nil {
		return nil, BadRequestError("Could not parse album id")
	}
	var param request
	if err = decodeRequest(r, &param); err != nil {
		return nil, err
	}
	rows, err := s.pg.Album.SetPhotos(id, param.PhotoIds)
	if err != nil {
		return nil, err
	}

	return AffectedItems{NumItems: rows}, err
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

func (s *mserver) handleUpdateOrder(r *http.Request) (interface{}, error) {
	type request struct {
		Photos []uuid.UUID
	}
	var param request
	var id uuid.UUID
	var err error
	if err = decodeRequest(r, &param); err != nil {
		return nil, err
	}
	id, err = uuid.Parse(Var(r, "albumid"))
	if err != nil {
		return nil, BadRequestError("Could not parse album id")
	}
	fmt.Println("This is number of photos in order: ", len(param.Photos))
	fmt.Println("This is photos: ", param.Photos)
	if s.pg.Album.Has(id) {
		return s.pg.Album.UpdateOrder(id, param.Photos)
	} else {
		return nil, NotFoundError("Album not found")
	}

}
