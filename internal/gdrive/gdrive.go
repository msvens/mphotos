package gdrive

import (
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"io"
	"os"
	"time"
)

const (
	ErrorBadRequest         = 400
	ErrorInvalidCredentials = 401
	ErrorLimitExceeded      = 403
	ErrorFileNotFound       = 404
	ErrorTooManyRequests    = 429
	ErrorBackendError       = 500
	ErrorUnknownError       = 501
)

type DriveService struct {
	service *drive.Service
	Root    *drive.File
}

/*
Drive API Scopes
*/
func FullScope() string {
	return drive.DriveScope
}

func ReadOnlyScope() string {
	return drive.DriveReadonlyScope
}

func ParseTime(tstr string) (time.Time, error) {
	return time.Parse(time.RFC3339, tstr)
}

func NewDriveService(token *oauth2.Token, config *oauth2.Config) (*DriveService, error) {
	ctx := context.Background()
	srv, err := drive.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx, token)))
	if err != nil {
		return nil, err
	}
	rf, err := srv.Files.Get("root").Do()
	if err != nil {
		return nil, err
	}
	return &DriveService{srv, rf}, nil
}

func (ds *DriveService) About(fields ...googleapi.Field) (*drive.About, error) {
	acall := ds.service.About.Get()
	if fields != nil {
		acall = acall.Fields(fields...)
	}
	about, err := acall.Do()
	if err != nil {
		return nil, err
	}
	return about, nil
}

func (ds *DriveService) Download(id string, path string) (int64, error) {
	resp, err := ds.service.Files.Get(id).Download()
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, resp.Body)
}

func (ds *DriveService) Get(id string) (*drive.File, error) {
	lcall := ds.service.Files.Get(id)
	return lcall.Do()
}

func (ds *DriveService) GetByName(name string, folder bool, trashed bool, fileFields string) (*drive.File, error) {
	q := NewQuery().Name().Eq(name)
	if folder {
		q = q.And().MimeType().Eq(Folder)
	}
	if !trashed {
		q = q.And().TrashedEq(false)
	}
	return ds.GetByQuery(q, fileFields)
}

func (ds *DriveService) GetByQuery(q *Query, fileFields string) (*drive.File, error) {
	lcall := ds.service.Files.List()
	if q.Err() != nil {
		return nil, q.Err()
	}
	if q.IsEmpty() {
		return nil, &googleapi.Error{Code: ErrorBadRequest, Message: "no query defined"}
	}
	lcall.Q(q.String())

	if fileFields != "" {
		fields := fmt.Sprintf("files(%s)", fileFields)
		lcall.Fields(googleapi.Field(fields))
	}
	lcall.PageSize(1)

	r, err := lcall.Do()
	if err != nil {
		return nil, err
	}
	if len(r.Files) == 0 {
		return nil, &googleapi.Error{Code: ErrorFileNotFound, Message: "File Not Found"}
	}
	return r.Files[0], nil
}

func (ds *DriveService) List(pageSize int64, parentId string, fileFields string) (*drive.FileList, error) {
	lcall := ds.service.Files.List()
	if pageSize > 0 {
		lcall.PageSize(pageSize)

	}
	if parentId != "" {
		lcall.Q(NewQuery().Parents().In(parentId).String())
	}
	fields := fmt.Sprintf("nextPageToken, files(%s)", fileFields)
	return lcall.Fields(googleapi.Field(fields)).Do()
}

func (ds *DriveService) ListAll(parentId string, fileFields string) ([]*drive.File, error) {

	if parentId == "" {
		return nil, &googleapi.Error{Code: ErrorBadRequest, Message: "no parent Id provided"}
	}
	return ds.SearchAll(NewQuery().Parents().In(parentId), fileFields)
}

func (ds *DriveService) SearchFolder(parentId string, query *Query, fileFields string) ([]*drive.File, error) {
	if parentId == "" || query.IsEmpty() {
		return nil, &googleapi.Error{Code: ErrorBadRequest, Message: "parentId or query is empty"}
	}
	query.And().Parents().In(parentId)
	return ds.SearchAll(query, fileFields)
}

func (ds *DriveService) SearchAll(q *Query, fileFields string) ([]*drive.File, error) {
	lcall := ds.service.Files.List()
	if q.Err() != nil {
		return nil, q.Err()
	}
	if !q.IsEmpty() {
		lcall.Q(q.String())
	}
	fields := fmt.Sprintf("nextPageToken, files(%s)", fileFields)
	lcall.Fields(googleapi.Field(fields))

	var fs []*drive.File
	nextToken := ""

	for {
		if nextToken != "" {
			lcall.PageToken(nextToken)
		}
		r, err := lcall.Do()
		if err != nil {
			return nil, err
		}

		fs = append(fs, r.Files...)
		nextToken = r.NextPageToken
		if nextToken == "" {
			break
		}
	}
	return fs, nil
}
