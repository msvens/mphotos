package server

import (
	"bytes"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/msvens/mimage/img"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"image"
	"io"
	"net/http"
	"strconv"
)

func (s *mserver) handleImg(pt config.PhotoType, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	//http.ServeFile(w, r, filepath.Join(dir, name))
	http.ServeFile(w, r, config.PhotoFilePath(pt, name))
}

func (s *mserver) handleEditImage(r *http.Request) (interface{}, error) {
	type request struct {
		Rotation int `json:"rotation"`
		X        int `json:"x"`
		Y        int `json:"y"`
		Width    int `json:"width"`
		Height   int `json:"height"`
	}
	id, err := uuid.Parse(Var(r, "photoid"))
	if err != nil {
		return nil, BadRequestError("no photo id")
	}
	var par request
	if err := decodeRequest(r, &par); err != nil {
		return nil, err
	}
	p, err := s.pg.Photo.Get(id)
	if err != nil {
		return nil, BadRequestError("could not find photo")
	}
	//Transform image:
	fname := config.PhotoFilePath(config.Original, p.FileName)
	srcImage, exifBytes, err := img.OpenOpts(fname, false, true)
	if err != nil {
		return nil, InternalError("Could not open image file")
	}
	if par.Rotation != 0 {
		srcImage = img.RotateImage(srcImage, par.Rotation)
	}
	rect := image.Rect(par.X, par.Y, par.X+par.Width, par.Y+par.Height)
	if !rect.Empty() {
		srcImage = img.CropImage(srcImage, rect)
	}
	//now save and crop
	err = img.SaveOpts(srcImage, fname, 90, exifBytes)
	if err != nil {
		return nil, InternalError("Could not save edited image file")
	}
	err = dao.GenerateImages(p.FileName)
	if err != nil {
		return nil, InternalError("Could not generate image versions")
	}
	return p, nil
}

func (s *mserver) handleEditPreviewImage(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Rotation int `json:"rotation"`
		X        int `json:"x"`
		Y        int `json:"y"`
		Width    int `json:"width"`
		Height   int `json:"height"`
	}

	if loggedIn := ctxLoggedIn(r.Context()); !loggedIn {
		http.Error(w, "user not logged in", http.StatusMethodNotAllowed)
		return
	}

	id, err := uuid.Parse(Var(r, "photoid"))
	if err != nil {
		http.Error(w, "Could not parse Id", http.StatusBadRequest)
		return
	}
	var par request
	if err := decodeRequest(r, &par); err != nil {
		http.Error(w, "Could not parse request", http.StatusBadRequest)
		return
	}
	//now create a preview image:
	p, err := s.pg.Photo.Get(id)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	fname := config.PhotoFilePath(config.Original, p.FileName)
	srcImage, err := img.Open(fname)
	if err != nil {
		http.Error(w, "could not open image file", http.StatusInternalServerError)
	}
	if par.Rotation != 0 {
		srcImage = img.RotateImage(srcImage, par.Rotation)
	}
	rect := image.Rect(par.X, par.Y, par.X+par.Width, par.Y+par.Height)
	if !rect.Empty() {
		srcImage = img.CropImage(srcImage, rect)
	}
	buffer := new(bytes.Buffer)
	imaging.Encode(buffer, srcImage, imaging.JPEG, imaging.JPEGQuality(90))
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(buffer.Len()))
	io.Copy(w, buffer)
	return
}

func (s *mserver) handleImage(w http.ResponseWriter, r *http.Request) {
	s.handleImg(config.Original, w, r)
}

func (s *mserver) handleResize(w http.ResponseWriter, r *http.Request) {
	s.handleImg(config.Resize, w, r)
}

func (s *mserver) handlePortrait(w http.ResponseWriter, r *http.Request) {
	s.handleImg(config.Portrait, w, r)
}

func (s *mserver) handleLandscape(w http.ResponseWriter, r *http.Request) {
	s.handleImg(config.Landscape, w, r)
}

func (s *mserver) handleSquare(w http.ResponseWriter, r *http.Request) {
	s.handleImg(config.Square, w, r)
}

func (s *mserver) handleThumb(w http.ResponseWriter, r *http.Request) {
	s.handleImg(config.Thumb, w, r)
}
