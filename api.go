package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/msvens/mdrive"
	"github.com/msvens/mphotos/service"
	"google.golang.org/api/drive/v3"
	"io"
	"log"
	"net/http"
	"os"

	"strconv"
)

type PSResponse struct {
	Err error `json:"error,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

type FolderResponse struct {
	Err error `json:"error,omitempty"`
	Folder *drive.File `json:"folder,omitempty"`
}

type JsonResponse struct {
	Error int `json:"error"`
	Desc string `json:"desc"`
}

var (
	ps *service.PhotoService
)

func InitApi(r *mux.Router, pp string) {

	//google drive calls
	r.Path(pp+"/drive/list").HandlerFunc(List)

	r.Path(pp+"/photos/folder").Methods("POST").HandlerFunc(SetPhotoFolder)
	r.Path(pp+"/photos/folder").Methods("GET").HandlerFunc(GetPhotoFolder)
	r.Path(pp+"/photos/folder/check").Methods("GET").HandlerFunc(CheckFolder)

	r.Path(pp+"/photos").Methods("GET").HandlerFunc(GetPhotos)
	r.Path(pp+"/photos").Methods("PUT", "POST").HandlerFunc(UpdatePhotos)

	r.Path(pp+"/photos/{id}/orig").Methods("Get").HandlerFunc(DownloadPhoto)
	r.Path(pp+"/photos/{id}/exif").Methods("Get").HandlerFunc(GetExif)
	r.Path(pp+"/photos/{id}").Methods("GET").HandlerFunc(GetPhoto)
	r.Path(pp+"/photos/{id}").Methods("DELETE").HandlerFunc(DeletePhoto)

	r.Path(pp+"/images/{name}").Methods("Get").HandlerFunc(GetImage)
	r.Path(pp+"/thumbs/{name}").Methods("Get").HandlerFunc(GetThumb)

}

func List(w http.ResponseWriter, r *http.Request){
	var fl []*drive.File
	var err error

	name := r.URL.Query().Get("name")
	if name != "" {
		fl, err = ps.ListDriveByName(name)
	} else {
		fl, err = ps.ListDriveById(getFolderId(r))
	}
	psResponse(fl, err, w)
}

func IsLoggedIn() bool {
	return ps != nil
}

func SetPhotoService(drvService *mdrive.DriveService) {
	ps = service.NewPhotosService(drvService)
}

func setJson(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func CheckFolder(w http.ResponseWriter, _ *http.Request) {
	fl, err := ps.ListNNewPhotos()
	psResponse(fl, err, w)
}

func GetPhotoFolder(w http.ResponseWriter, _ *http.Request) {
	folder := ps.GetPhotoFolder()
	if folder == nil {
		e := mdrive.NewError(mdrive.ErrorBackendError, "no photofolder set")
		psResponse(nil, e, w)
		return
	}
	psResponse(folder, nil, w)
}

func SetPhotoFolder(w http.ResponseWriter, r *http.Request) {
	err :=r.ParseForm()
	if err != nil {
		log.Println("could not parse form: ", err)
		jsonResponse(w, http.StatusBadRequest, err.Error())
	}
	folderName := r.FormValue("name")
	if folderName == "" {
		//googleapi.Error{gdrive.ErrorBadRequest, "missing form value name"}
		jsonResponse(w, http.StatusBadRequest, "missing form value name")
	}
	f, err := ps.SetDriveFolderByName(folderName)
	if err != nil {
		errResponse(w, err)
	}
	okResponse(w, "folder set to: "+f.Name)
}

func GetExif(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if exif, found := ps.GetExif(id); found {
		psResponse(exif, nil, w)
	} else {
		psResponse(nil, mdrive.NewError(mdrive.ErrorBadRequest, "exif does not exist"), w)
	}

}

func GetPhoto(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	photo, found := ps.GetPhoto(id)
	if !found {
		psResponse(nil, mdrive.NewError(mdrive.ErrorBadRequest, "photo does not exist"), w)
	} else {
		psResponse(photo, nil, w)
	}
}

func DeletePhoto(w http.ResponseWriter, _ *http.Request) {
	psResponse(nil, mdrive.NewError(mdrive.ErrorBackendError, "Function not yet implemented"), w)
}

func GetImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	http.ServeFile(w, r, ps.GetImgPath(name))
}

func GetThumb(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	http.ServeFile(w, r, ps.GetThumbPath(name))
}

func DownloadPhoto(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	p, f := ps.GetPhoto(id)
	if !f {
		http.Error(w, "file not found", http.StatusNotFound)
	}
	file, err := os.Open(ps.GetImgPath(p.FileName))
	if err != nil {
		fmt.Println("error in file download")
		//File not found, send 404
		http.Error(w, "File not found.", http.StatusNotFound)
		return
	}
	defer file.Close() //Close after function return
	FileHeader := make([]byte, 512)

	//Copy the headers into the FileHeader buffer
	file.Read(FileHeader)

	//Get content type of file
	FileContentType := http.DetectContentType(FileHeader)

	fmt.Println(FileContentType)
	//Get the file size
	FileStat, _ := file.Stat()                     //Get info from file
	FileSize := strconv.FormatInt(FileStat.Size(), 10) //Get file size as a string

	//Send the headers
	//w.Header().Set("Content-Disposition", "attachment; filename="+path.Base(p.Path))
	w.Header().Set("Content-Type", FileContentType)
	w.Header().Set("Content-Length", FileSize)

	//Send the file
	//We read 512 bytes from the file already, so we reset the offset back to 0
	file.Seek(0, 0)
	io.Copy(w, file) //'Copy' the file to the client
	return

}

func GetPhotos(w http.ResponseWriter, r *http.Request) {
	if photos, err := ps.GetPhotos(); err == nil {
		psResponse(photos, nil, w)
	} else {
		psResponse(nil, err, w)
	}
}

func UpdatePhotos(w http.ResponseWriter, r*http.Request) {
	err := ps.AddPhotos()
	if err != nil {
		psResponse(nil, err, w)
	} else {
		psResponse("Folder Updated", nil, w)
	}
}

func jsonResponse(w http.ResponseWriter, code int, message string) {
	resp := JsonResponse{code, message}
	setJson(w)

	//we prefix for debug purpose for now
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	err := enc.Encode(resp)
	if err != nil {
		log.Println(err)
	}
}
func errResponse(w http.ResponseWriter, err error) {
	jsonResponse(w, http.StatusInternalServerError, err.Error())
}

func okResponse(w http.ResponseWriter, msg string) {
	jsonResponse(w, http.StatusOK, msg)
}

func psResponse(data interface{}, err error, w http.ResponseWriter) {
	setJson(w)
	enc := json.NewEncoder(w)
	resp := PSResponse{err, data}
	enc.SetIndent("", "  ")
	e := enc.Encode(resp)
	if e != nil {
		log.Println(e)
	}
}

func getFolderId(r *http.Request) string {
	vars := mux.Vars(r)
	folderId := vars["id"]
	if folderId == "" {
		return ps.DriveSrv.Root.Id
	} else {
		return folderId
	}
}

