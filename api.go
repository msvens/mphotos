package main

import (
	"encoding/gob"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/msvens/mphotos/config"
	"github.com/msvens/mphotos/service"
	"go.uber.org/zap"
	"io"
	"net/http"
	"os"
	"strconv"
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
	r.Path(pp + "/drive/authenticated").Methods("GET").HandlerFunc(ah(Authenticated))
	r.Path(pp + "/drive/auth").Methods("GET").HandlerFunc(HandleGoogleLogin)
	r.Path(pp + "/drive/check").Methods("GET").HandlerFunc(ah(CheckDriveFolder))

	r.Path(pp + "/login").Methods("POST").HandlerFunc(hw(Login))
	r.Path(pp + "/logout").Methods("GET").HandlerFunc(hw(Logout))
	r.Path(pp + "/loggedin").Methods("GET").HandlerFunc(hw(LoggedIn))

	r.Path(pp + "/photos").Methods("GET").HandlerFunc(h(GetPhotos))
	r.Path(pp + "/photos/search").Methods("GET").HandlerFunc(h(SearchPhotos))
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
	r.Path(pp + "/user/pic").Methods("PUT").HandlerFunc(ah(UpdateUserPic))
	r.Path(pp + "/user/drive").Methods("PUT").HandlerFunc(ah(UpdateUserDrive))

	r.Path(pp + "/images/{name}").Methods("Get").HandlerFunc(GetImage)
	r.Path(pp + "/thumbs/{name}").Methods("Get").HandlerFunc(GetThumb)
}

/******API FUNCTIONS***********************************/
func Authenticated(r *http.Request) (interface{}, error) {
	return AuthUser{isGoogleConnected()}, nil
}

func GetExif(r *http.Request) (interface{}, error) {
	if exif, found := ps.GetExif(Var(r, "id")); found {
		return exif, nil
	} else {
		return nil, service.NotFoundError("exif does not exist")
	}
}

func GetLatestPhoto(_ *http.Request) (interface{}, error) {
	if photo, found := ps.GetLatestPhoto(); found {
		return photo, nil
	} else {
		return nil, service.NotFoundError("photo does not exist")
	}
}

func GetPhoto(r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	id := vars["id"]
	if photo, found := ps.GetPhoto(id); found {
		return photo, nil
	} else {
		return nil, service.NotFoundError("photo does not exist")
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

type SearchPhotosParam struct {
	CameraModel string
	FocalLength string
	Title       string
	Keywords    string
	Description string
	Generic     string
}

func SearchPhotos(r *http.Request) (interface{}, error) {
	var params SearchPhotosParam
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	if params.CameraModel != "" {
		return ps.SearchByCameraModel(params.CameraModel)
	} else {
		return nil, service.InternalError("Search pattern not yet implemented")
	}
}

type GetPhotosParam struct {
	Limit        int
	Offset       int
	OriginalDate bool
}

func GetPhotos(r *http.Request) (interface{}, error) {
	var params GetPhotosParam
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	} else {
		return ps.GetPhotos(params.OriginalDate, params.Limit, params.Offset)
	}
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

type LoginParams struct {
	Password string `json:"password" schema:"password"`
}

func Login(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	session, err := store.Get(r, cookieName)
	if err != nil {
		return nil, service.InternalError(err.Error())
	}
	var loginParams LoginParams
	if err = decodeRequest(r, &loginParams); err != nil {
		return nil, err
	} else if loginParams.Password != config.ServicePassword() {
		if e := session.Save(r, w); e != nil {
			return nil, service.InternalError(err.Error())
		}
		return nil, service.UnauthorizedError("Incorret user password")
	}
	user := &AuthUser{true}
	session.Values["user"] = user
	if err := session.Save(r, w); err != nil {
		return nil, service.InternalError(err.Error())
	}
	return user, nil
}

/*
func Login2(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if session, err := store.Get(r, cookieName); err != nil {
		return nil, service.InternalError(err.Error())
	} else if r.FormValue("password") != config.ServicePassword() {
		if err := session.Save(r, w); err != nil {
			return nil, service.InternalError(err.Error())
		}
		return nil, service.UnauthorizedError("This code was incorrect")
	} else {
		user := &AuthUser{true}
		session.Values["user"] = user
		if err := session.Save(r, w); err != nil {
			return nil, service.InternalError(err.Error())
		}
		return user, nil
	}
}
*/

func Logout(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if session, err := store.Get(r, cookieName); err != nil {
		return nil, service.InternalError(err.Error())
	} else {
		session.Values["user"] = AuthUser{}
		session.Options.MaxAge = -1
		if err := session.Save(r, w); err != nil {
			return nil, service.InternalError(err.Error())
		}
		return session.Values["user"], nil
	}
}

func LoggedIn(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return AuthUser{isLoggedIn(w, r)}, nil
}

type DeletePhotoParam struct {
	RemoveFiles bool `json:"removeFiles" schema:"removeFiles"`
}

func DeletePhoto(r *http.Request) (interface{}, error) {
	photo, found := ps.GetPhoto(Var(r, "id"))
	if !found {
		return nil, service.NotFoundError("photo not found")
	}
	var params DeletePhotoParam
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	return ps.DeletePhoto(photo, params.RemoveFiles)
}

func DeletePhotos(r *http.Request) (interface{}, error) {
	var params DeletePhotoParam
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	return ps.DeletePhotos(params.RemoveFiles)
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
	var ep EditPhoto
	if err := decodeRequest(r, &ep); err != nil {
		return nil, err
	}
	return ps.UpdatePhoto(ep.Id, ep.Title, ep.Description, ep.Keywords)
}

func UpdateUserPic(r *http.Request) (interface{}, error) {
	var u service.User
	if err := decodeRequest(r, &u); err != nil {
		return nil, err
	}
	return ps.UpdateUserPic(u.Pic)
}

func UpdateUserDrive(r *http.Request) (interface{}, error) {
	var u service.User
	if err := decodeRequest(r, &u); err != nil {
		return nil, err
	}
	return ps.UpdateUserDrive(u.DriveFolderName)
}

func UpdateUser(r *http.Request) (interface{}, error) {
	var u service.User
	if err := decodeRequest(r, &u); err != nil {
		return nil, err
	}
	return ps.UpdateUser(&u)
}
