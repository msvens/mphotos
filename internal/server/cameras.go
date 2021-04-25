package server

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/msvens/mphotos/internal/gdrive"
	"github.com/msvens/mphotos/internal/img"
	"github.com/msvens/mphotos/internal/model"
	"io"
	"net/http"
	"os"
)

var cameraSizes []int = []int{48, 192, 512}

func (s *mserver) handleCamera(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	id := Var(r, "id")
	return s.db.Camera(id)
}

func (s *mserver) handleCameras(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return s.db.Cameras()
}

func (s *mserver) handleUpdateCamera(r *http.Request) (interface{}, error) {
	//id := Var(r, "id")
	var params model.Camera
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	fmt.Println("update camera: ", params.Id, " ", params.Year)
	return s.db.UpdateCamera(&params)
}

func (s *mserver) handleCameraImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	var size int
	var imgPath string
	if s, ok := vars["size"]; ok {
		if s == "48" {
			size = 48
		} else if s == "192" {
			size = 192
		} else if s == "512" {
			size = 512
		} else {
			http.Error(w, "No Such Image Size", http.StatusNotFound)
			return
		}
	}
	if camera, err := s.db.Camera(id); err != nil {
		http.Error(w, "No Such Camera", http.StatusNotFound)
	} else if camera.Image == "" {
		http.Error(w, "No Camera Image", http.StatusNotFound)
	} else {
		if size > 0 {
			imgPath = cameraPath(s, fmt.Sprint(id, "-", size, camera.Image))
		} else {
			imgPath = cameraPath(s, fmt.Sprint(id, camera.Image))
		}
		http.ServeFile(w, r, imgPath)
	}
}

func (s *mserver) uploadCameraImageFromFile(r *http.Request) (interface{}, error) {
	id := Var(r, "id")
	r.ParseMultipartForm(10 << 20) //10M
	file, _, err := r.FormFile("cameraImage")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	//Detect type
	buff := make([]byte, 512)
	_, err = file.Read(buff)
	if err != nil {
		return nil, err
	}
	mt := http.DetectContentType(buff)
	if mt != gdrive.Jpeg && mt != gdrive.Png {
		return nil, BadRequestError("Images is not of the correct mimetype: " + mt)
	}
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	ext := ".jpg"
	if mt == gdrive.Png {
		ext = ".png"
	}
	fileName := id + ext
	dst, err := os.Create(cameraPath(s, fileName))
	defer dst.Close()
	_, err = io.Copy(dst, file)
	if err != nil {
		return nil, err
	}
	src := imgPath(s, fileName)
	sizes := make(map[string]int)
	for _, size := range cameraSizes {
		sizes[cameraPath(s, fmt.Sprint(id, "-", size, ext))] = size
	}
	if err = img.ResizeImages(src, sizes); err != nil {
		return nil, err
	}
	return s.db.UpdateCameraImage(ext, id)
}

func (s *mserver) uploadCameraImageFromURL(r *http.Request) (interface{}, error) {
	id := Var(r, "id")
	if !s.db.HasCamera(id) {
		return nil, NotFoundError("Camera not found: " + id)
	}
	type request struct {
		Url string
	}
	var params request
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	response, err := http.Get(params.Url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	mt := response.Header.Get("Content-Type")
	if mt != gdrive.Jpeg && mt != gdrive.Png {
		return nil, BadRequestError("Image is not of the correct mimetype: " + mt)
	}
	ext := ".jpg"
	if mt == gdrive.Png {
		ext = ".png"
	}
	fileName := id + ext
	file, err := os.Create(cameraPath(s, fileName))
	defer file.Close()
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return nil, err
	}
	src := imgPath(s, fileName)
	sizes := make(map[string]int)
	for _, size := range cameraSizes {
		sizes[cameraPath(s, fmt.Sprint(id, "-", size, ext))] = size
	}
	if err = img.ResizeImages(src, sizes); err != nil {
		return nil, err
	}
	return s.db.UpdateCameraImage(ext, id)
}
