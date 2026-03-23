package server

import "net/http"

func (s *mserver) routes() {

	s.mGET("/albums", s.loginInfo(s.handleAlbums))
	s.mPUT("/albums", s.authOnly(s.handleAddAlbum))
	s.mGET("/albums/names/{name}", s.loginInfo(s.handleAlbumByName))
	s.mGET("/albums/{albumid}", s.loginInfo(s.handleAlbum))
	s.mDELETE("/albums/{albumid}", s.authOnly(s.handleDeleteAlbum))
	s.mPUT("/albums/{albumid}", s.authOnly(s.handleUpdateAlbum))
	s.mPUT("/albums/{albumid}/order", s.authOnly(s.handleUpdateOrder))
	s.mGET("/albums/{albumid}/photos", s.mResponse(s.handleAlbumPhotos))
	s.mPUT("/albums/{albumid}/photos/add", s.authOnly(s.handleAddAlbumPhotos))
	s.mPUT("/albums/{albumid}/photos/clear", s.authOnly(s.handleClearAlbumPhotos))
	s.mPUT("/albums/{albumid}/photos/delete", s.authOnly(s.handleDeleteAlbumPhotos))
	s.mPUT("/albums/{albumid}/photos/set", s.authOnly(s.handleSetAlbumPhotos))

	s.r.HandleFunc(s.prefixPath+"/auth/callback", s.handleGoogleCallback)

	s.mGET("/cameras", s.mResponse(s.handleCameras))
	s.mGET("/cameras/{cameraid}", s.mResponse(s.handleCamera))
	s.mPUT("/cameras/{cameraid}", s.authOnly(s.handleUpdateCamera))
	s.mGET("/cameras/{cameraid}/image/{size}", s.handleCameraImage)
	s.mGET("/cameras/{cameraid}/image", s.handleCameraImage)
	s.mPUT("/cameras/{cameraid}/image/upload", s.authOnly(s.uploadCameraImageFromFile))
	s.mPUT("/cameras/{cameraid}/image", s.authOnly(s.uploadCameraImageFromURL))

	s.mGET("/drive/search", s.authOnly(s.handleSearchDrive))
	s.mGET("/drive", s.authOnly(s.handleDrive))
	s.mGET("/drive/authenticated", s.authOnly(s.handleAuthenticatedDrive))
	s.mGET("/drive/disconnect", s.authOnly(s.handleDisconnectDrive))
	s.mGET("/drive/auth", s.handleGoogleLogin)
	s.mGET("/drive/check", s.authOnly(s.handleCheckDrive))
	s.mPUT("/drive/upload", s.authOnly(s.handleAddDrivePhotos))
	s.mPUT("/drive/job/schedule", s.authOnly(s.handleScheduleDriveJob))
	s.mGET("/drive/job/{jobid}", s.authOnly(s.handleStatusDriveJob))

	s.mPUT("/local/upload", s.authOnly(s.handleUploadLocalPhoto))
	s.mPUT("/local/check", s.authOnly(s.handleCheckLocalPhotos))

	s.mGET("/images/{name}", s.handleImage)
	s.mGET("/thumbs/{name}", s.handleThumb)
	s.mGET("/squares/{name}", s.handleSquare)
	s.mGET("/portraits/{name}", s.handlePortrait)
	s.mGET("/landscapes/{name}", s.handleLandscape)
	s.mGET("/resizes/{name}", s.handleResize)

	s.mPUT("/login", s.mResponse(s.handleLogin))
	s.mGET("/logout", s.mResponse(s.handleLogout))
	s.mGET("/loggedin", s.loginInfo(s.handleLoggedIn))

	s.mGET("/photos", s.authOnly(s.handlePhotos))
	s.mDELETE("/photos", s.authOnly(s.handleDeletePhotos))
	s.mGET("/photos/{photoid}/albums", s.loginInfo(s.handlePhotoAlbums))
	s.mPUT("/photos/{photoid}/albums/add", s.authOnly(s.handleAddPhotoAlbums))
	s.mPUT("/photos/{photoid}/albums/delete", s.authOnly(s.handleDeletePhotoAlbums))
	s.mPUT("/photos/{photoid}/albums/clear", s.authOnly(s.handleClearPhotoAlbums))
	s.mPUT("/photos/{photoid}/albums/set", s.authOnly(s.handleSetPhotoAlbums))
	s.mGET("/photos/{photoid}/orig", s.handleDownloadPhoto)
	s.mGET("/photos/{photoid}/exif", s.mResponse(s.handleExif))
	s.mGET("/photos/{photoid}/edit/preview", s.handleEditPreviewImage)
	s.mPUT("/photos/{photoid}/edit", s.authOnly(s.handleEditImage))
	s.mGET("/photos/{photoid}", s.mResponse(s.handlePhoto))
	s.mPUT("/photos/{photoid}", s.authOnly(s.handleUpdatePhoto))
	s.mDELETE("/photos/{photoid}", s.authOnly(s.handleDeletePhoto))

	s.mPUT("/comments/{img}", s.guestOnly(s.handleCommentPhoto))
	s.mGET("/comments/{img}", s.loginInfo(s.handlePhotoComments))
	s.mPUT("/guest", s.mResponse(s.handleCreateGuest))
	s.mGET("/guest", s.guestOnly(s.handleGuest))
	s.mPUT("/guest/update", s.guestOnly(s.handleUpdateGuest))
	s.mGET("/guest/logout", s.mResponse(s.handleLogoutGuest))
	s.mGET("/guest/is", s.mResponse(s.handleIsGuest))
	s.mGET("/guest/likes", s.guestOnly(s.handleGuestLikes))
	s.mGET("/guest/likes/{photoid}", s.guestOnly(s.handleGuestLikePhoto))
	s.mGET("/guest/verify", s.mResponse(s.handleVerifyGuest))
	s.mPUT("/likes/{photoid}/like", s.guestOnly(s.handleLikePhoto))
	s.mPUT("/likes/{photoid}/unlike", s.guestOnly(s.handleUnlikePhoto))
	s.mGET("/likes/{photoid}", s.loginInfo(s.handlePhotoLikes))
	s.mGET("/user", s.loginInfo(s.handleUser))
	s.mPUT("/user", s.authOnly(s.handleUpdateUser))
	s.mPUT("/user/pic", s.authOnly(s.handleUpdatePicUser))
	s.mPUT("/user/gdrive", s.authOnly(s.handleUpdateDriveUser))
	s.mGET("/user/config", s.mResponse(s.handleUserConfig))
	s.mPUT("/user/config", s.authOnly(s.handleUpdateConfig))
}

func (s *mserver) mGET(p string, h http.HandlerFunc) {
	s.r.HandleFunc("GET "+s.prefixPath+p, h)
}

func (s *mserver) mDELETE(p string, h http.HandlerFunc) {
	s.r.HandleFunc("DELETE "+s.prefixPath+p, h)
}

func (s *mserver) mPUT(p string, h http.HandlerFunc) {
	s.r.HandleFunc("PUT "+s.prefixPath+p, h)
	s.r.HandleFunc("POST "+s.prefixPath+p, h)
}
