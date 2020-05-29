package main

import (
	"encoding/gob"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/msvens/mdrive"
	"github.com/msvens/mphotos/config"
	"github.com/msvens/mphotos/service"
	"go.uber.org/zap"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var (
	ps         *service.PhotoService
	store      *sessions.CookieStore
	cookieName string
)

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
	r.Path(pp+"/photos/job/schedule").Methods("PUT", "POST").HandlerFunc(ah(ScheduleJob))
	r.Path(pp + "/photos/job/{id}").Methods("GET").HandlerFunc(ah(StatusJob))
	r.Path(pp + "/photos").Methods("DELETE").HandlerFunc(ah(DeletePhotos))

	r.Path(pp + "/photos/{id}/orig").Methods("GET").HandlerFunc(DownloadPhoto)
	r.Path(pp + "/photos/{id}/exif").Methods("GET").HandlerFunc(h(GetExif))
	r.Path(pp + "/photos/latest").Methods("GET").HandlerFunc(h(GetLatestPhoto))
	r.Path(pp + "/photos/{id}").Methods("GET").HandlerFunc(h(GetPhoto))
	r.Path(pp+"/photos/{id}").Methods("POST", "PUT").HandlerFunc(ah(UpdatePhoto))
	r.Path(pp + "/photos/{id}").Methods("DELETE").HandlerFunc(ah(DeletePhoto))

	r.Path(pp + "/user").Methods("GET").HandlerFunc(hw(GetUser))
	r.Path(pp+"/user").Methods("POST", "PUT").HandlerFunc(ah(UpdateUser))

	r.Path(pp + "/images/{name}").Methods("Get").HandlerFunc(GetImage)
	r.Path(pp + "/thumbs/{name}").Methods("Get").HandlerFunc(GetThumb)
}

func SetPhotoService(drvService *mdrive.DriveService) {
	if s, err := service.NewPhotosService(drvService); err != nil {
		logger.Panicw("could not create photos service", zap.Error(err))
	} else {
		ps = s
	}
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

func GetLatestPhoto(_ *http.Request) (interface{}, error) {
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
		logger.Infow("could not download file", zap.Error(err))
		http.Error(w, "File not found.", http.StatusNotFound)
		return
	}
	defer file.Close() //Close after function return
	FileHeader := make([]byte, 512)

	//Copy the headers into the FileHeader buffer
	file.Read(FileHeader)

	//Get content type of file
	FileContentType := http.DetectContentType(FileHeader)

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
	auth := isLoggedIn(w, r)
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
	return AuthUser{isLoggedIn(w, r)}, nil
}

func DeletePhoto(r *http.Request) (interface{}, error) {
	id := Var(r, "id")
	photo, found := ps.GetPhoto(id)
	if !found {
		return nil, service.NewError(service.ApiErrorNotFound, "photo not found")
	}
	form(r)
	_, found = r.Form["removeFiles"]
	return ps.DeletePhoto(photo, found)
}

func CheckDriveFolder(_ *http.Request) (interface{}, error) {
	return ps.CheckPhotos()
}

func ListDrive(_ *http.Request) (interface{}, error) {
	return ps.ListDrive()
}

func SearchDrive(r *http.Request) (interface{}, error) {
	name := r.URL.Query().Get("name")
	id := r.URL.Query().Get("id")
	return ps.SearchDrive(id, name)
}

func UpdateDriveFolder(r *http.Request) (interface{}, error) {
	form(r)
	return ps.UpdateDriveFolder(r.Form.Get("name"))
}

func DeletePhotos(r *http.Request) (interface{}, error) {
	form(r)
	rem := r.Form.Get("removeFiles")
	removeFiles := false
	if strings.ToLower(rem) == "true" {
		removeFiles = true
	}
	return ps.DeletePhotos(removeFiles)
}

func StatusJob(r *http.Request) (interface{}, error) {
	return ps.JobStatus(Var(r, "id"))
}

func ScheduleJob(_ *http.Request) (interface{}, error) {
	return ps.ScheduleAddPhotos()
}

func UpdatePhotos(_ *http.Request) (interface{}, error) {
	return ps.AddPhotos()
}

func UpdatePhoto(r *http.Request) (interface{}, error) {
	form(r)
	return ps.UpdatePhoto(r.Form, Var(r, "id"))
}

func UpdateUser(r *http.Request) (interface{}, error) {
	form(r)
	var u = service.User{Name: r.Form.Get("name"), Bio: r.Form.Get("bio"), Pic: r.Form.Get("pic")}
	var fields []string
	if r.Form.Get("columns") != "" {
		fields = strings.Split(r.Form.Get("columns"), ",")
	}
	usr, err := ps.UpdateUser(&u, fields)
	return usr, err

}
