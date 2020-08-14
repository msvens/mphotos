package server

import (
	"github.com/msvens/mdrive"
	"google.golang.org/api/drive/v3"
	"net/http"
	"time"
)

const (
	fileFields = "id, name, kind, mimeType, md5Checksum, createdTime"
)

type DriveFile struct {
	CreatedTime time.Time
	Id          string `json:"id"`
	Kind        string `json:"kind"`
	Md5Checksum string `json:"md5Checksum"`
	MimeType    string `json:"mimeType"`
}

type DriveFiles struct {
	Length int          `json:"length"`
	Files  []*DriveFile `json:"files,omitempty"`
}

func (s *mserver) handleSearchDrive(r *http.Request) (interface{}, error) {
	name := r.URL.Query().Get("name")
	id := r.URL.Query().Get("id")
	if files, err := searchDriveFiles(s, id, name); err != nil {
		return nil, err
	} else {
		return toDriveFiles(files), nil
	}
}

func (s *mserver) handleDrive(_ *http.Request) (interface{}, error) {
	if files, err := listDriveFiles(s); err != nil {
		return nil, err
	} else {
		return toDriveFiles(files), nil
	}
}

func (s *mserver) handleAuthenticatedDrive(r *http.Request) (interface{}, error) {
	return AuthUser{s.isGoogleConnected()}, nil
}

func (s *mserver) handleCheckDrive(_ *http.Request) (interface{}, error) {
	if files, err := checkPhotosDrive(s); err != nil {
		return nil, err
	} else {
		return toDriveFiles(files), nil
	}
}

func checkPhotosDrive(s *mserver) ([]*drive.File, error) {
	fl, err := listDriveFiles(s)
	if err != nil {
		return nil, err
	}
	var ret []*drive.File
	for _, f := range fl {
		if !s.db.HasPhoto(f.Id, true) {
			ret = append(ret, f)
		}
	}
	return ret, nil
}

func listDriveFiles(s *mserver) ([]*drive.File, error) {
	if u, err := s.db.User(); err != nil {
		return nil, InternalError("user not found")
	} else if u.DriveFolderId == "" {
		return nil, NotFoundError("Drive folder has not been set")
	} else {
		return searchDriveFiles(s, u.DriveFolderId, "")
	}
}

func searchDriveFiles(s *mserver, id string, name string) ([]*drive.File, error) {
	if name != "" {
		if f, err := s.ds.GetByName(name, true, false, fileFields); err != nil {
			return nil, err
		} else {
			id = f.Id
		}
	}
	query := mdrive.NewQuery().Parents().In(id).And().MimeType().Eq(mdrive.Jpeg).TrashedEq(false)
	return s.ds.SearchAll(query, fileFields)
}

func toDriveFile(file *drive.File) *DriveFile {
	df := DriveFile{
		Id:          file.Id,
		Kind:        file.Kind,
		Md5Checksum: file.Md5Checksum,
		MimeType:    file.MimeType,
	}
	df.CreatedTime, _ = mdrive.ParseTime(file.CreatedTime)
	return &df
}

func toDriveFiles(files []*drive.File) *DriveFiles {
	ret := DriveFiles{Length: len(files)}
	if ret.Length > 0 {
		for _, f := range files {
			ret.Files = append(ret.Files, toDriveFile(f))
		}
	}
	return &ret
}
