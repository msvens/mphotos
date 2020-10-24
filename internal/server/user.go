package server

import (
	"encoding/json"
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

func (s *mserver) handleUserConfig(_ http.ResponseWriter, r *http.Request) (interface{}, error) {
	if c, err := s.db.UserConfig(); err == nil {
		var conf map[string]interface{}
		if err := json.Unmarshal([]byte(c), &conf); err != nil {
			return nil, err
		} else {
			return &conf, nil
		}
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

func (s *mserver) handleUpdateConfig(r *http.Request) (interface{}, error) {
	var c map[string]interface{}

	if err := decodeRequest(r, &c); err != nil {
		return nil, err
	}
	if b, err := json.Marshal(c); err != nil {
		return nil, err
	} else {
		return c, s.db.UpdateUserConfig(string(b))
	}
}
