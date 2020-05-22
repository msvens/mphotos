package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/msvens/mdrive"
	"github.com/msvens/mphotos/config"
	"github.com/msvens/mphotos/service"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type HttpHandler func(http.ResponseWriter, *http.Request)
type ReqHandler func(r *http.Request) (interface{}, error)
type ReqRespHandler func(w http.ResponseWriter, r *http.Request) (interface{}, error)

type PSResponse struct {
	Err  *service.ApiError `json:"error,omitempty"`
	Data interface{}       `json:"data,omitempty"`
}

type AuthUser struct {
	Authenticated bool `json:"authenticated"`
}

var (
	ps         *service.PhotoService
	store      *sessions.CookieStore
	cookieName string
)

//ah decorates a function with session checks and outputs mphotos json
//ah should be used for any function that you need to be logged in to the api for
func ah(f ReqHandler) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		if checkAndWrite(w, r) {
			data, err := f(r)
			psResponse(data, err, w)
		}
	}
}

//h decorates a function to output result as mphotos json format
func h(f ReqHandler) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := f(r)
		psResponse(data, err, w)
	}
}

//hw decorates a function to output result as mphotos json format
func hw(f ReqRespHandler) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := f(w, r)
		psResponse(data, err, w)
	}
}

func checkLogin(w http.ResponseWriter, r *http.Request) error {
	session, err := store.Get(r, cookieName)
	if err != nil {
		return service.NewError(service.ApiErrorBackendError, err.Error())
	}
	user := getSessionUser(session)
	if !user.Authenticated {
		err = session.Save(r, w)
		if err != nil {
			return service.NewError(service.ApiErrorBackendError, err.Error())
		}
		return service.NewError(service.ApiErrorInvalidCredentials, "user not authenticated to api")
	}
	return nil
}

func checkAndWrite(w http.ResponseWriter, r *http.Request) bool {
	if err := checkLogin(w, r); err != nil {
		psResponse(nil, err, w)
		return false
	}
	return true
}

func isPhotosLogin(w http.ResponseWriter, r *http.Request) bool {
	if err := checkLogin(w, r); err == nil {
		return true
	}
	return false
}

func isGoogleLoggedIn() bool {
	return ps != nil
}

func getSessionUser(s *sessions.Session) AuthUser {
	val := s.Values["user"]
	var user = AuthUser{}
	user, ok := val.(AuthUser)
	if !ok {
		return AuthUser{Authenticated: false}
	}
	return user
}

func setJson(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func psResponse(data interface{}, err error, w http.ResponseWriter) {
	setJson(w)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	var resp PSResponse
	if err != nil {
		resp = PSResponse{service.ResolveError(err), nil}
	} else {
		resp = PSResponse{nil, data}
	}
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

func InitApi(r *mux.Router, pp string) {

	//Initialize session
	authKeyOne := []byte(config.SessionAuthcKey())
	encKeyOne := []byte(config.SessionEncKey())
	cookieName = config.SessionCookieName()

	store = sessions.NewCookieStore(
		authKeyOne,
		encKeyOne,
	)

	store.Options = &sessions.Options{
		MaxAge:   60 * 60 * 24,
		HttpOnly: true,
	}

	gob.Register(AuthUser{})

	//register routes
	r.Path(pp + "/drive/search").Methods("GET").HandlerFunc(ah(SearchDrive))
	r.Path(pp + "/drive").Methods("GET").HandlerFunc(ah(ListDrive))
	r.Path(pp + "/drive/check").Methods("GET").HandlerFunc(ah(CheckDriveFolder))
	r.Path(pp+"/drive").Methods("POST", "PUT").HandlerFunc(ah(UpdateDriveFolder))

	r.Path(pp + "/login").Methods("POST").HandlerFunc(hw(Login))
	r.Path(pp + "/logout").Methods("GET").HandlerFunc(hw(Logout))
	r.Path(pp + "/loggedin").Methods("GET").HandlerFunc(hw(LoggedIn))

	r.Path(pp + "/photos").Methods("GET").HandlerFunc(h(GetPhotos))
	r.Path(pp+"/photos").Methods("PUT", "POST").HandlerFunc(ah(UpdatePhotos))
	r.Path(pp + "/photos").Methods("DELETE").HandlerFunc(ah(DeletePhotos))

	r.Path(pp + "/photos/{id}/orig").Methods("GET").HandlerFunc(DownloadPhoto)
	r.Path(pp + "/photos/{id}/exif").Methods("GET").HandlerFunc(h(GetExif))
	r.Path(pp + "/photos/latest").Methods("GET").HandlerFunc(h(GetLatestPhoto))
	r.Path(pp + "/photos/{id}").Methods("GET").HandlerFunc(h(GetPhoto))
	r.Path(pp + "/photos/{id}").Methods("DELETE").HandlerFunc(ah(DeletePhoto))

	r.Path(pp + "/user").Methods("GET").HandlerFunc(hw(GetUser))
	r.Path(pp+"/user").Methods("POST", "PUT").HandlerFunc(ah(UpdateUser))

	r.Path(pp + "/images/{name}").Methods("Get").HandlerFunc(GetImage)
	r.Path(pp + "/thumbs/{name}").Methods("Get").HandlerFunc(GetThumb)
}

func SetPhotoService(drvService *mdrive.DriveService) {
	ps = service.NewPhotosService(drvService)
}

/******API FUNCTIONS***********************************/

func GetExif(r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	id := vars["id"]
	if exif, found := ps.GetExif(id); found {
		return exif, nil
	} else {
		return nil, service.NewError(service.ApiErrorBadRequest, "exif does not exist")
	}
}

func GetLatestPhoto(r *http.Request) (interface{}, error) {
	if photo, found := ps.GetLatestPhoto(); found {
		return photo, nil
	} else {
		return nil, service.NewError(service.ApiErrorNotFound, "photo does not exist")
	}
}

func GetPhoto(r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	id := vars["id"]
	if photo, found := ps.GetPhoto(id); found {
		return photo, nil
	} else {
		return nil, service.NewError(service.ApiErrorNotFound, "photo does not exist")
	}
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
	FileStat, _ := file.Stat()                         //Get info from file
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

func GetPhotos(r *http.Request) (interface{}, error) {

	query := r.URL.Query()
	limit := 1000
	offset := 0
	driveDate := true
	if q := query.Get("limit"); q != "" {
		limit, _ = strconv.Atoi(q)
	}
	if q := query.Get("offset"); q != "" {
		offset, _ = strconv.Atoi(q)
	}
	if _, f := query["originalDate"]; f {
		driveDate = false
	}
	return ps.GetPhotos(driveDate, limit, offset)

}

func GetUser(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	auth := isPhotosLogin(w, r)
	if u, err := ps.GetUser(); err == nil {
		if !auth {
			u.DriveFolderId = ""
			u.DriveFolderName = ""
		}
		return u, nil
	} else {
		return nil, err
	}
}

func Login(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if session, err := store.Get(r, cookieName); err != nil {
		return nil, service.NewError(service.ApiErrorBackendError, err.Error())
	} else if r.FormValue("password") != config.ServicePassword() {
		if err := session.Save(r, w); err != nil {
			return nil, service.NewError(service.ApiErrorBackendError, err.Error())
		}
		return nil, service.NewError(service.ApiErrorInvalidCredentials, "This code was incorrect")
	} else {
		user := &AuthUser{true}
		session.Values["user"] = user
		if err := session.Save(r, w); err != nil {
			return nil, service.NewError(service.ApiErrorBackendError, err.Error())
		}
		return user, nil
	}
}

func Logout(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if session, err := store.Get(r, cookieName); err != nil {
		return nil, service.NewError(service.ApiErrorBackendError, err.Error())
	} else {
		session.Values["user"] = AuthUser{}
		session.Options.MaxAge = -1
		if err := session.Save(r, w); err != nil {
			return nil, service.NewError(service.ApiErrorBackendError, err.Error())
		}
		return session.Values["user"], nil
	}
}

func LoggedIn(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return AuthUser{isPhotosLogin(w, r)}, nil
}

func DeletePhoto(r *http.Request) (interface{}, error) {
	return "", service.NewError(service.ApiErrorBackendError, "Function not yet implemented")
}

func CheckDriveFolder(r *http.Request) (interface{}, error) {
	return ps.CheckPhotos()
}

func ListDrive(r *http.Request) (interface{}, error) {
	return ps.ListDrive()
}

func SearchDrive(r *http.Request) (interface{}, error) {
	name := r.URL.Query().Get("name")
	return ps.SearchDrive(getFolderId(r), name)
}

func UpdateDriveFolder(r *http.Request) (interface{}, error) {

	/*
		err :=r.ParseForm()
		if err != nil {
			return nil, service.NewError(service.ApiErrorBadRequest, err.Error())
		}*/
	folderName := r.FormValue("name")
	if folderName == "" {
		return nil, service.NewError(service.ApiErrorBadRequest, "missing form value name")
	}
	return ps.UpdateDriveFolder("", folderName)
}

func DeletePhotos(r *http.Request) (interface{}, error) {
	fmt.Println("in delete")
	rem := r.FormValue("removeFiles")
	removeFiles := false
	if strings.ToLower(rem) == "true" {
		removeFiles = true
	}
	return ps.DeletePhotos(removeFiles)
}

func UpdatePhotos(r *http.Request) (interface{}, error) {
	return ps.AddPhotos()
}

func UpdateUser(r *http.Request) (interface{}, error) {

	/*
		if err := r.ParseForm(); err != nil {
			return nil, mdrive.NewError(mdrive.ErrorBadRequest, err.Error())
		}*/

	name := r.FormValue("name")
	pic := r.FormValue("pic")
	bio := r.FormValue("bio")
	var u = service.User{Name: name, Bio: bio, Pic: pic}
	var fields []string
	if r.FormValue("columns") != "" {
		fields = strings.Split(r.FormValue("columns"), ",")
	}
	usr, err := ps.UpdateUser(&u, fields)
	return usr, err

}
