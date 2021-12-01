package server

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/msvens/mimage/img"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"github.com/msvens/mphotos/internal/gdrive"
	"io"
	"net/http"
	"os"
)

//var cameraSizes []int = []int{48, 192, 512}
var cameraSizes = []img.Options{
	img.NewOptions(img.Resize, 48, 0, false),
	img.NewOptions(img.Resize, 192, 0, false),
	img.NewOptions(img.Resize, 512, 0, false),
}

func (s *mserver) handleCamera(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	id := Var(r, "id")
	return s.pg.Camera.Get(id)
}

func (s *mserver) handleCameras(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return s.pg.Camera.List()
}

func (s *mserver) handleUpdateCamera(r *http.Request) (interface{}, error) {
	var params dao.Camera
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	return s.pg.Camera.Update(&params)
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
	if camera, err := s.pg.Camera.Get(id); err != nil {
		http.Error(w, "No Such Camera", http.StatusNotFound)
	} else if camera.Image == "" {
		http.Error(w, "No Camera Image", http.StatusNotFound)
	} else {
		fname := fmt.Sprint(id, camera.Image)
		if size > 0 {
			fname = fmt.Sprint(id, "-", size, camera.Image)
		}
		http.ServeFile(w, r, config.CameraFilePath(fname))
		/*if size > 0 {
			imgPath = cameraPath(s, fmt.Sprint(id, "-", size, camera.Image))
		} else {
			imgPath = cameraPath(s, fmt.Sprint(id, camera.Image))
		}*/

		http.ServeFile(w, r, imgPath)
	}
}

func (s *mserver) uploadCameraImageFromFile(r *http.Request) (interface{}, error) {
	id := Var(r, "id")

	if !s.pg.Camera.Has(id) {
		return nil, NotFoundError("Camera not found: " + id)
	}
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
	//dst, err := os.Create(cameraPath(s, fileName))
	dst, err := os.Create(config.CameraFilePath(fileName))
	defer dst.Close()
	_, err = io.Copy(dst, file)
	if err != nil {
		return nil, err
	}
	//src := cameraPath(s, fileName)
	src := config.CameraFilePath(fileName)
	sizes := make(map[string]img.Options)
	for _, size := range cameraSizes {
		sizes[config.CameraFilePath(fmt.Sprint(id, "-", size, ext))] = size
		//sizes[cameraPath(s, fmt.Sprint(id, "-", size, ext))] = size
	}

	if err = img.TransformFile(src, sizes); err != nil {
		return nil, err
	}
	return s.pg.Camera.UpdateImage(ext, id)
}

func (s *mserver) uploadCameraImageFromURL(r *http.Request) (interface{}, error) {
	id := Var(r, "id")
	if !s.pg.Camera.Has(id) {
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
	//file, err := os.Create(cameraPath(s, fileName))
	file, err := os.Create(config.CameraFilePath(fileName))
	defer file.Close()
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return nil, err
	}
	//src := cameraPath(s, fileName)
	src := config.CameraFilePath(fileName)
	sizes := make(map[string]img.Options)
	for _, size := range cameraSizes {
		sizes[config.CameraFilePath(fmt.Sprint(id, "-", size, ext))] = size
		//sizes[cameraPath(s, fmt.Sprint(id, "-", size, ext))] = size
	}
	if err = img.TransformFile(src, sizes); err != nil {
		return nil, err
	}
	return s.pg.Camera.UpdateImage(ext, id)
}
