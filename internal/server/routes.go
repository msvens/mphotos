package server

import "github.com/gorilla/mux"

func (s *mserver) routes() {

	s.r.Use(s.userGuestInfoMW)

	s.mGET("/albums").HandlerFunc(s.loginInfo(s.handleAlbums))
	s.mPUT("/albums").HandlerFunc(s.authOnly(s.handleAddAlbum))
	s.mGET("/albums/names/{name}").HandlerFunc(s.loginInfo(s.handleAlbumByName))
	s.mGET("/albums/{albumid}").HandlerFunc(s.loginInfo(s.handleAlbum))
	s.mDELETE("/albums/{albumid}").HandlerFunc(s.authOnly(s.handleDeleteAlbum))
	s.mPUT("/albums/{albumid}").HandlerFunc(s.authOnly(s.handleUpdateAlbum))
	s.mPUT("/albums/{albumid}/order").HandlerFunc(s.authOnly(s.handleUpdateOrder))
	s.mGET("/albums/{albumid}/photos").HandlerFunc(s.mResponse(s.handleAlbumPhotos))
	s.mPUT("/albums/{albumid}/photos/add").HandlerFunc(s.authOnly(s.handleAddAlbumPhotos))
	s.mPUT("/albums/{albumid}/photos/clear").HandlerFunc(s.authOnly(s.handleClearAlbumPhotos))
	s.mPUT("/albums/{albumid}/photos/delete").HandlerFunc(s.authOnly(s.handleDeleteAlbumPhotos))
	s.mPUT("/albums/{albumid}/photos/set").HandlerFunc(s.authOnly(s.handleSetAlbumPhotos))

	//s.path("/auth/login").HandlerFunc(s.handleGoogleLogin)
	s.path("/auth/callback").HandlerFunc(s.handleGoogleCallback)

	s.mGET("/cameras").HandlerFunc(s.mResponse(s.handleCameras))
	s.mGET("/cameras/{cameraid}").HandlerFunc(s.mResponse(s.handleCamera))
	s.mPUT("/cameras/{cameraid}").HandlerFunc(s.authOnly(s.handleUpdateCamera))
	s.mGET("/cameras/{cameraid}/image/{size}").HandlerFunc(s.handleCameraImage)
	s.mGET("/cameras/{cameraid}/image").HandlerFunc(s.handleCameraImage)
	s.mPUT("/cameras/{cameraid}/image/upload").HandlerFunc(s.authOnly(s.uploadCameraImageFromFile))
	s.mPUT("/cameras/{cameraid}/image").HandlerFunc(s.authOnly(s.uploadCameraImageFromURL))

	s.mGET("/drive/search").HandlerFunc(s.authOnly(s.handleSearchDrive))
	s.mGET("/drive").HandlerFunc(s.authOnly(s.handleDrive))
	s.mGET("/drive/authenticated").HandlerFunc(s.authOnly(s.handleAuthenticatedDrive))
	s.mGET("/drive/disconnect").HandlerFunc(s.authOnly(s.handleDisconnectDrive))
	s.mGET("/drive/auth").HandlerFunc(s.handleGoogleLogin)
	s.mGET("/drive/check").HandlerFunc(s.authOnly(s.handleCheckDrive))
	s.mPUT("/drive/upload").HandlerFunc(s.authOnly(s.handleAddDrivePhotos))
	s.mPUT("/drive/job/schedule").HandlerFunc(s.authOnly(s.handleScheduleDriveJob))
	s.mGET("/drive/job/{jobid}").HandlerFunc(s.authOnly(s.handleStatusDriveJob))

	s.mPUT("/local/upload").HandlerFunc(s.authOnly(s.handleUploadLocalPhoto))
	s.mPUT("/local/check").HandlerFunc(s.authOnly(s.handleCheckLocalPhotos))

	s.mGET("/images/{name}").HandlerFunc(s.handleImage)
	s.mGET("/thumbs/{name}").HandlerFunc(s.handleThumb)
	s.mGET("/squares/{name}").HandlerFunc(s.handleSquare)
	s.mGET("/portraits/{name}").HandlerFunc(s.handlePortrait)
	s.mGET("/landscapes/{name}").HandlerFunc(s.handleLandscape)
	s.mGET("/resizes/{name}").HandlerFunc(s.handleResize)

	s.mPUT("/login").HandlerFunc(s.mResponse(s.handleLogin))
	s.mGET("/logout").HandlerFunc(s.mResponse(s.handleLogout))
	s.mGET("/loggedin").HandlerFunc(s.loginInfo(s.handleLoggedIn))

	s.mGET("/photos").HandlerFunc(s.authOnly(s.handlePhotos))
	//s.mGET("/photos/search").HandlerFunc(s.loginInfo(s.handleSearchPhotos))
	s.mDELETE("/photos").HandlerFunc(s.authOnly(s.handleDeletePhotos))
	s.mGET("/photos/{photoid}/albums").HandlerFunc(s.loginInfo(s.handlePhotoAlbums))
	s.mPUT("/photos/{photoid}/albums/add").HandlerFunc(s.authOnly(s.handleAddPhotoAlbums))
	s.mPUT("/photos/{photoid}/albums/delete").HandlerFunc(s.authOnly(s.handleDeletePhotoAlbums))
	s.mPUT("/photos/{photoid}/albums/clear").HandlerFunc(s.authOnly(s.handleClearPhotoAlbums))
	s.mPUT("/photos/{photoid}/albums/set").HandlerFunc(s.authOnly(s.handleSetPhotoAlbums))
	s.mGET("/photos/{photoid}/orig").HandlerFunc(s.handleDownloadPhoto)
	s.mGET("/photos/{photoid}/exif").HandlerFunc(s.mResponse(s.handleExif))
	s.mGET("/photos/{photoid}/edit/preview").HandlerFunc(s.handleEditPreviewImage)
	s.mPUT("/photos/{photoid}/edit").HandlerFunc(s.authOnly(s.handleEditImage))
	//s.path("/photos/latest").Methods("GET").HandlerFunc(s.loginInfo(s.handleLatestPhoto))
	s.mGET("/photos/{photoid}").HandlerFunc(s.mResponse(s.handlePhoto))
	s.mPUT("/photos/{photoid}").HandlerFunc(s.authOnly(s.handleUpdatePhoto))
	s.mDELETE("/photos/{photoid}").HandlerFunc(s.authOnly(s.handleDeletePhoto))
	//s.path("/photos/{id}/private").Methods("POST", "PUT").HandlerFunc(s.authOnly(s.handleUpdatePhotoPrivate))

	s.mPUT("/comments/{img}").HandlerFunc(s.guestOnly(s.handleCommentPhoto))
	s.mGET("/comments/{img}").HandlerFunc(s.loginInfo(s.handlePhotoComments))
	s.mPUT("/guest").HandlerFunc(s.mResponse(s.handleCreateGuest))
	s.mGET("/guest").HandlerFunc(s.guestOnly(s.handleGuest))
	s.mPUT("/guest/update").HandlerFunc(s.guestOnly(s.handleUpdateGuest))
	s.mGET("/guest/logout").HandlerFunc(s.mResponse(s.handleLogoutGuest))
	s.mGET("/guest/is").HandlerFunc(s.mResponse(s.handleIsGuest))
	s.mGET("/guest/likes").HandlerFunc(s.guestOnly(s.handleGuestLikes))
	s.mGET("/guest/likes/{photoid}").HandlerFunc(s.guestOnly(s.handleGuestLikePhoto))
	s.mGET("/guest/verify").HandlerFunc(s.mResponse(s.handleVerifyGuest))
	s.mPUT("/likes/{photoid}/like").HandlerFunc(s.guestOnly(s.handleLikePhoto))
	s.mPUT("/likes/{photoid}/unlike").HandlerFunc(s.guestOnly(s.handleUnlikePhoto))
	s.mGET("/likes/{photoid}").HandlerFunc(s.loginInfo(s.handlePhotoLikes))
	s.mGET("/user").HandlerFunc(s.loginInfo(s.handleUser))
	s.mPUT("/user").HandlerFunc(s.authOnly(s.handleUpdateUser))
	s.mPUT("/user/pic").HandlerFunc(s.authOnly(s.handleUpdatePicUser))
	s.mPUT("/user/gdrive").HandlerFunc(s.authOnly(s.handleUpdateDriveUser))
	s.mGET("/user/config").HandlerFunc(s.mResponse(s.handleUserConfig))
	s.mPUT("/user/config").HandlerFunc(s.authOnly(s.handleUpdateConfig))
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
