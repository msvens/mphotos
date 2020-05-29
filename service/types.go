package service

import (
	"fmt"
	"github.com/msvens/mdrive"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"time"
)

const (
	ApiErrorBadRequest         = 400
	ApiErrorInvalidCredentials = 401
	ApiErrorLimitExceeded      = 403
	ApiErrorNotFound           = 404
	ApiErrorTooManyRequests    = 429
	ApiErrorBackendError       = 500
	ApiErrorUnknownError       = 501
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

type types struct{}

func (e *ApiError) Error() string {
	return fmt.Sprintf("code: %d message: %s", e.Code, e.Message)
}

func NewError(code int, message string) *ApiError {
	return &ApiError{Code: code, Message: message}
}

func NewBadRequest(message string) *ApiError {
	return NewError(ApiErrorBadRequest, message)
}

func NewBackendError(message string) *ApiError {
	return NewError(ApiErrorBackendError, message)
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
	return &ApiError{ApiErrorBackendError, err.Error()}
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
