package server

import (
	"encoding/json"
	"github.com/msvens/mphotos/internal/dao"
	"net/http"
)

func (s *mserver) handleUser(r *http.Request, loggedIn bool) (interface{}, error) {
	if u, err := s.pg.User.Get(); err == nil {
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
	if u, err := s.pg.User.Get(); err == nil {
		var conf map[string]interface{}
		if err := json.Unmarshal([]byte(u.Config), &conf); err != nil {
			return nil, err
		} else {
			return &conf, nil
		}
	} else {
		return nil, err
	}
}

func (s *mserver) handleUpdatePicUser(r *http.Request) (interface{}, error) {
	var u dao.User
	if err := decodeRequest(r, &u); err != nil {
		return nil, err
	}
	if user, err := s.pg.User.Get(); err != nil {
		return nil, InternalError(err.Error())
	} else {
		user.Pic = u.Pic
		return s.pg.User.Update(user)
	}
	//return s.ps.UpdateUserPic(u.Pic)
}

func (s *mserver) handleUpdateDriveUser(r *http.Request) (interface{}, error) {
	var u dao.User
	if err := decodeRequest(r, &u); err != nil {
		return nil, err
	}
	if f, err := s.ds.GetByName(u.DriveFolderName, true, false, fileFields); err != nil {
		return nil, err
	} else {
		if user, err := s.pg.User.Get(); err != nil {
			return nil, InternalError(err.Error())
		} else {
			user.DriveFolderId = f.Id
			user.DriveFolderName = f.Name
			return s.pg.User.Update(user)
		}
	}
	//return s.ps.UpdateUserDrive(u.DriveFolderName)
}

func (s *mserver) handleUpdateUser(r *http.Request) (interface{}, error) {
	var u dao.User
	if err := decodeRequest(r, &u); err != nil {
		return nil, err
	}
	if user, err := s.pg.User.Get(); err != nil {
		return nil, InternalError(err.Error())
	} else {
		user.Bio = u.Bio
		user.Pic = u.Pic
		user.Name = u.Name
		return s.pg.User.Update(user)
	}
}

func (s *mserver) handleUpdateConfig(r *http.Request) (interface{}, error) {
	var c map[string]interface{}

	if err := decodeRequest(r, &c); err != nil {
		return nil, err
	}
	if b, err := json.Marshal(c); err != nil {
		return nil, err
	} else {
		if user, err := s.pg.User.Get(); err != nil {
			return nil, InternalError(err.Error())
		} else {
			user.Config = string(b)
			return s.pg.User.Update(user)
		}
	}
}
