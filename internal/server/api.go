package server

import (
	"encoding/gob"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/service"
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
	r.Path(pp + "/drive/search").Methods("GET").HandlerFunc(arh(SearchDrive))
	r.Path(pp + "/drive").Methods("GET").HandlerFunc(arh(ListDrive))
	r.Path(pp + "/drive/authenticated").Methods("GET").HandlerFunc(arh(Authenticated))
	r.Path(pp + "/drive/auth").Methods("GET").HandlerFunc(HandleGoogleLogin)
	r.Path(pp + "/drive/check").Methods("GET").HandlerFunc(arh(CheckDriveFolder))

	r.Path(pp + "/login").Methods("POST").HandlerFunc(rwh(Login))
	r.Path(pp + "/logout").Methods("GET").HandlerFunc(rwh(Logout))
	r.Path(pp + "/loggedin").Methods("GET").HandlerFunc(lrh(LoggedIn))

	r.Path(pp + "/albums").Methods("GET").HandlerFunc(rh(GetAlbums))
	r.Path(pp + "/albums/{name}").Methods("GET").HandlerFunc(lrh(GetAlbum))
	r.Path(pp+"/albums/{name}").Methods("PUT", "POST").HandlerFunc(arh(UpdateAlbum))

	r.Path(pp + "/photos").Methods("GET").HandlerFunc(lrh(GetPhotos))
	r.Path(pp + "/photos/search").Methods("GET").HandlerFunc(lrh(SearchPhotos))
	r.Path(pp+"/photos").Methods("PUT", "POST").HandlerFunc(arh(UpdatePhotos))
	r.Path(pp+"/photos/job/schedule").Methods("PUT", "POST").HandlerFunc(arh(ScheduleJob))
	r.Path(pp + "/photos/job/{id}").Methods("GET").HandlerFunc(arh(StatusJob))
	r.Path(pp + "/photos").Methods("DELETE").HandlerFunc(arh(DeletePhotos))

	r.Path(pp + "/photos/{id}/albums").Methods("GET").HandlerFunc(lrh(GetPhotoAlbums))
	r.Path(pp + "/photos/{id}/orig").Methods("GET").HandlerFunc(DownloadPhoto)
	r.Path(pp + "/photos/{id}/exif").Methods("GET").HandlerFunc(lrh(GetExif))
	r.Path(pp + "/photos/latest").Methods("GET").HandlerFunc(lrh(GetLatestPhoto))
	r.Path(pp + "/photos/{id}").Methods("GET").HandlerFunc(lrh(GetPhoto))
	r.Path(pp+"/photos/{id}").Methods("POST", "PUT").HandlerFunc(arh(UpdatePhoto))
	r.Path(pp + "/photos/{id}").Methods("DELETE").HandlerFunc(arh(DeletePhoto))
	r.Path(pp+"/photos/{id}/private").Methods("POST", "PUT").HandlerFunc(arh(UpdatePrivate))

	r.Path(pp + "/user").Methods("GET").HandlerFunc(lrh(GetUser))
	r.Path(pp+"/user").Methods("POST", "PUT").HandlerFunc(arh(UpdateUser))
	r.Path(pp + "/user/pic").Methods("PUT").HandlerFunc(arh(UpdateUserPic))
	r.Path(pp + "/user/drive").Methods("PUT").HandlerFunc(arh(UpdateUserDrive))

	r.Path(pp + "/images/{name}").Methods("Get").HandlerFunc(GetImage)
	r.Path(pp + "/thumbs/{name}").Methods("Get").HandlerFunc(GetThumb)
}

/******API FUNCTIONS***********************************/
func Authenticated(r *http.Request) (interface{}, error) {
	return AuthUser{isGoogleConnected()}, nil
}

func GetAlbum(r *http.Request, loggedIn bool) (interface{}, error) {
	vars := mux.Vars(r)
	name := vars["name"]
	return ps.GetAlbumCollection(name, loggedIn)
}

func GetAlbums(r *http.Request) (interface{}, error) {
	return ps.GetAlbums()
}

func GetExif(r *http.Request, loggedIn bool) (interface{}, error) {
	if exif, found := ps.GetExif(Var(r, "id"), loggedIn); found {
		return exif, nil
	} else {
		return nil, service.NotFoundError("exif does not exist")
	}
}

func GetPhotoAlbums(r *http.Request, loggedIn bool) (interface{}, error) {
	vars := mux.Vars(r)
	id := vars["id"]
	return ps.GetPhotoAlbums(id, loggedIn)
}

func GetLatestPhoto(_ *http.Request, loggedIn bool) (interface{}, error) {
	if photo, found := ps.GetLatestPhoto(loggedIn); found {
		return photo, nil
	} else {
		return nil, service.NotFoundError("photo does not exist")
	}
}

func GetPhoto(r *http.Request, loggedIn bool) (interface{}, error) {
	vars := mux.Vars(r)
	id := vars["id"]
	if photo, found := ps.GetPhoto(id, loggedIn); found {
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
	loggedIn := isLoggedIn(w, r)
	p, f := ps.GetPhoto(id, loggedIn)
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

func SearchPhotos(r *http.Request, loggedIn bool) (interface{}, error) {
	var params SearchPhotosParam
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	if params.CameraModel != "" {
		return ps.SearchByCameraModel(params.CameraModel, loggedIn)
	} else {
		return nil, service.InternalError("Search pattern not yet implemented")
	}
}

type GetPhotosParam struct {
	Limit        int
	Offset       int
	OriginalDate bool
}

func GetPhotos(r *http.Request, loggedIn bool) (interface{}, error) {
	var params GetPhotosParam
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	} else {
		return ps.GetPhotos(params.OriginalDate, params.Limit, params.Offset, loggedIn)
	}
}

func GetUser(r *http.Request, loggedIn bool) (interface{}, error) {
	if u, err := ps.GetUser(); err == nil {
		if !loggedIn {
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

func LoggedIn(r *http.Request, loggedIn bool) (interface{}, error) {
	return AuthUser{loggedIn}, nil
}

type DeletePhotoParam struct {
	RemoveFiles bool `json:"removeFiles" schema:"removeFiles"`
}

func DeletePhoto(r *http.Request) (interface{}, error) {
	photo, found := ps.GetPhoto(Var(r, "id"), true)
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

func UpdateAlbum(r *http.Request) (interface{}, error) {
	var a service.Album
	println("in update album")
	if err := decodeRequest(r, &a); err != nil {
		return nil, err
	}
	return ps.UpdateAlbum(a.Description, a.CoverPic, a.Name)
}

type EditPhoto struct {
	Id          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
	Albums      []string `json:"albums"`
}

func UpdatePhoto(r *http.Request) (interface{}, error) {
	var ep EditPhoto
	if err := decodeRequest(r, &ep); err != nil {
		return nil, err
	}
	return ps.UpdatePhoto(ep.Id, ep.Title, ep.Description, ep.Keywords, ep.Albums)
}

func UpdatePrivate(r *http.Request) (interface{}, error) {
	photo, found := ps.GetPhoto(Var(r, "id"), true)
	if !found {
		return nil, service.NotFoundError("photo not found")
	}
	return ps.TogglePrivate(photo)
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
