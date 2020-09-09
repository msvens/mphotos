package server

import "github.com/gorilla/mux"

func (s *mserver) routes() {

	s.mGET("/albums").HandlerFunc(s.mResponse(s.handleAlbums))
	s.mGET("/albums/{name}").HandlerFunc(s.loginInfo(s.handleAlbum))
	s.mDELETE("/albums/{name}").HandlerFunc(s.authOnly(s.handleDeleteAlbum))
	s.mPUT("/albums/{name}").HandlerFunc(s.authOnly(s.handleUpdateAlbum))

	s.path("/auth/login").HandlerFunc(s.handleGoogleLogin)
	s.path("/uth/callback").HandlerFunc(s.handleGoogleCallback)

	s.path("/drive/search").Methods("GET").HandlerFunc(s.authOnly(s.handleSearchDrive))
	s.path("/drive").Methods("GET").HandlerFunc(s.authOnly(s.handleDrive))
	s.path("/drive/authenticated").Methods("GET").HandlerFunc(s.authOnly(s.handleAuthenticatedDrive))
	s.path("/drive/auth").Methods("GET").HandlerFunc(s.handleGoogleLogin)
	s.path("/drive/check").Methods("GET").HandlerFunc(s.authOnly(s.handleCheckDrive))

	s.path("/images/{name}").Methods("Get").HandlerFunc(s.handleImage)
	s.path("/thumbs/{name}").Methods("Get").HandlerFunc(s.handleThumb)
	s.path("/squares/{name}").Methods("Get").HandlerFunc(s.handleSquare)
	s.path("/portraits/{name}").Methods("Get").HandlerFunc(s.handlePortrait)
	s.path("/landscapes/{name}").Methods("Get").HandlerFunc(s.handleLandscape)
	s.path("/resizes/{image}").Methods("Get").HandlerFunc(s.handleResize)

	s.path("/login").Methods("POST").HandlerFunc(s.mResponse(s.handleLogin))
	s.path("/logout").Methods("GET").HandlerFunc(s.mResponse(s.handleLogout))
	s.path("/loggedin").Methods("GET").HandlerFunc(s.loginInfo(s.handleLoggedIn))

	s.mGET("/photos").HandlerFunc(s.loginInfo(s.handlePhotos))
	s.mGET("/photos/search").HandlerFunc(s.loginInfo(s.handleSearchPhotos))
	s.path("/photos").Methods("PUT", "POST").HandlerFunc(s.authOnly(s.handleUpdatePhotos))
	s.path("/photos/job/schedule").Methods("PUT", "POST").HandlerFunc(s.authOnly(s.handleScheduleJob))
	s.path("/photos/job/{id}").Methods("GET").HandlerFunc(s.authOnly(s.handleStatusJob))
	s.path("/photos").Methods("DELETE").HandlerFunc(s.authOnly(s.handleDeletePhotos))
	s.path("/photos/{id}/albums").Methods("GET").HandlerFunc(s.loginInfo(s.handlePhotoAlbums))
	s.path("/photos/{id}/orig").Methods("GET").HandlerFunc(s.handleDownloadPhoto)
	s.path("/photos/{id}/exif").Methods("GET").HandlerFunc(s.loginInfo(s.handleExif))
	s.path("/photos/latest").Methods("GET").HandlerFunc(s.loginInfo(s.handleLatestPhoto))
	s.path("/photos/{id}").Methods("GET").HandlerFunc(s.loginInfo(s.handlePhoto))
	s.path("/photos/{id}").Methods("POST", "PUT").HandlerFunc(s.authOnly(s.handleUpdatePhoto))
	s.path("/photos/{id}").Methods("DELETE").HandlerFunc(s.authOnly(s.handleDeletePhoto))
	s.path("/photos/{id}/private").Methods("POST", "PUT").HandlerFunc(s.authOnly(s.handleUpdatePhotoPrivate))

	s.path("/user").Methods("GET").HandlerFunc(s.loginInfo(s.handleUser))
	s.path("/user").Methods("POST", "PUT").HandlerFunc(s.authOnly(s.handleUpdateUser))
	s.path("/user/pic").Methods("PUT").HandlerFunc(s.authOnly(s.handleUpdatePicUser))
	s.path("/user/drive").Methods("PUT").HandlerFunc(s.authOnly(s.handleUpdateDriveUser))
}

func (s *mserver) mGET(p string) *mux.Route {
	return s.path(p).Methods("GET")
}

func (s *mserver) mDELETE(p string) *mux.Route {
	return s.path(p).Methods("DELETE")
}

func (s *mserver) mPUT(p string) *mux.Route {
	return s.path(p).Methods("PUT", "POST")
}

func (s *mserver) path(path string) *mux.Route {
	return s.r.Path(s.prefixPath + path)
}
