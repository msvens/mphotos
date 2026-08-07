package server

import (
	"crypto/md5"
	"fmt"
	"github.com/google/uuid"
	"github.com/msvens/mimage/metadata"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"go.uber.org/zap"
	"io"
	"net/http"
	"os"
	"time"
)

func calcMD5(src io.Reader) (string, error) {
	var ret string
	hash := md5.New()
	if _, err := io.Copy(hash, src); err != nil {
		return ret, err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func (s *mserver) handleUploadLocalPhoto(r *http.Request) (interface{}, error) {

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return nil, err
	}
	file, head, err := r.FormFile("image")
	if err != nil {
		return nil, err
	}

	//get some information about the file
	md5str, err := calcMD5(file)
	if err != nil {
		return nil, err
	}

	if s.pg.Photo.HasMd5(md5str) {
		return nil, BadRequestError("Photo already exists")
	}

	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	// Detect the format by content (mimage covers jpeg/png/gif/bmp/tiff).
	buff := make([]byte, 512)
	if _, err = file.Read(buff); err != nil {
		return nil, err
	}
	srcFormat := metadata.DetectFormat(buff)
	if !srcFormat.Supported() {
		return nil, BadRequestError("unsupported image type: " + srcFormat.String())
	}

	sourceId := r.FormValue("sourceId")
	if sourceId == "" {
		sourceId = head.Filename
	}
	sourceDate := time.Now()
	if sourceDateStr := r.FormValue("sourceDate"); sourceDateStr != "" {
		if d, err := time.Parse(time.RFC3339, sourceDateStr); err == nil {
			sourceDate = d
		} else {
			s.l.Warnw("could not parse sourceDate", zap.Error(err))
		}
	}

	photo := dao.Photo{}
	photo.Id = uuid.New()
	photo.Source = dao.SourceLocal
	photo.SourceId = sourceId
	photo.Md5 = md5str
	photo.SourceDate = sourceDate
	photo.UploadDate = time.Now()
	photo.FileName = photo.Id.String() + ".jpg"

	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	// Stage the upload to a temp file; finalizeImport converts it to the stored
	// <uuid>.jpg (or moves it into place if already jpeg) and handles metadata/variants.
	stagedPath := config.PhotoFilePath(config.Original, photo.Id.String()+".import")
	dst, err := os.Create(stagedPath)
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(dst, file); err != nil {
		_ = dst.Close()
		return nil, err
	}
	_ = dst.Close()

	if err := s.finalizeImport(&photo, stagedPath, srcFormat); err != nil {
		return nil, err
	}

	s.l.Infow("added img", "Id", photo.Id, "SourceId", photo.SourceId)
	return &photo, nil
}

func (s *mserver) handleCheckLocalPhotos(r *http.Request) (interface{}, error) {

	return nil, nil
}
