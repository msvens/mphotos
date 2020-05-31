package service

import (
	"fmt"
	"github.com/msvens/mdrive"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"net/http"
	"time"
)

type ApiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

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

type PhotoFiles struct {
	Length int      `json:"length"`
	Photos []*Photo `json:"photos,omitempty"`
}

func (e *ApiError) Error() string {
	return fmt.Sprintf("code: %d message: %s", e.Code, e.Message)
}

func newError(code int, message string) *ApiError {
	return &ApiError{Code: code, Message: message}
}

func UnauthorizedError(message string) *ApiError {
	return newError(http.StatusUnauthorized, message)
}

func NotFoundError(message string) *ApiError {
	return newError(http.StatusNotFound, message)
}

func BadRequestError(message string) *ApiError {
	return newError(http.StatusBadRequest, message)
}

func InternalError(message string) *ApiError {
	return newError(http.StatusInternalServerError, message)
}

func ResolveError(err error) *ApiError {
	e, ok := err.(*ApiError)
	if ok {
		return e
	}
	e1, ok := err.(*googleapi.Error)
	if ok {
		return &ApiError{e1.Code, e1.Message}
	}
	return InternalError(err.Error())
}

func ToDriveFile(file *drive.File) *DriveFile {
	df := DriveFile{
		Id:          file.Id,
		Kind:        file.Kind,
		Md5Checksum: file.Md5Checksum,
		MimeType:    file.MimeType,
	}
	df.CreatedTime, _ = mdrive.ParseTime(file.CreatedTime)
	return &df
}

func ToDriveFiles(files []*drive.File) *DriveFiles {
	ret := DriveFiles{Length: len(files)}
	if ret.Length > 0 {
		for _, f := range files {
			ret.Files = append(ret.Files, ToDriveFile(f))
		}
	}
	return &ret
}
