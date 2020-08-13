package server

import (
	"github.com/msvens/mphotos/internal/model"
	"net/http"
)

func (s *mserver) handleUser(r *http.Request, loggedIn bool) (interface{}, error) {
	if u, err := s.db.User(); err == nil {
		if !loggedIn {
			u.DriveFolderId = ""
			u.DriveFolderName = ""
		}
		return u, nil
	} else {
		return nil, err
	}
}

func (s *mserver) handleUpdatePicUser(r *http.Request) (interface{}, error) {
	var u model.User
	if err := decodeRequest(r, &u); err != nil {
		return nil, err
	}
	if user, err := s.db.User(); err != nil {
		return nil, InternalError(err.Error())
	} else {
		user.Pic = u.Pic
		return s.db.UpdateUser(user)
	}
	//return s.ps.UpdateUserPic(u.Pic)
}

func (s *mserver) handleUpdateDriveUser(r *http.Request) (interface{}, error) {
	var u model.User
	if err := decodeRequest(r, &u); err != nil {
		return nil, err
	}
	if f, err := s.ds.GetByName(u.DriveFolderName, true, false, fileFields); err != nil {
		return nil, err
	} else {
		if user, err := s.db.User(); err != nil {
			return nil, InternalError(err.Error())
		} else {
			user.DriveFolderId = f.Id
			user.DriveFolderName = f.Name
			return s.db.UpdateUser(user)
		}
	}
	//return s.ps.UpdateUserDrive(u.DriveFolderName)
}

func (s *mserver) handleUpdateUser(r *http.Request) (interface{}, error) {
	var u model.User
	if err := decodeRequest(r, &u); err != nil {
		return nil, err
	}
	return s.db.UpdateUser(&u)
	//return s.ps.UpdateUser(&u)
}
