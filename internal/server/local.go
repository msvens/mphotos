package server

import (
	"crypto/md5"
	"fmt"
	"github.com/google/uuid"
	"github.com/msvens/mimage/metadata"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"github.com/msvens/mphotos/internal/gdrive"
	"go.uber.org/zap"
	"io"
	"net/http"
	"os"
	"strings"
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

	r.ParseMultipartForm(10 << 20) //10M
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

	//Detect type
	buff := make([]byte, 512)
	_, err = file.Read(buff)
	if err != nil {
		return nil, err
	}
	mt := http.DetectContentType(buff)
	if mt != gdrive.Jpeg {
		return nil, BadRequestError("Image is not of the correct mimetype: " + mt)
	}

	sourceId := r.FormValue("sourceId")
	if sourceId == "" {
		sourceId = head.Filename
	}
	sourceDateStr := r.FormValue("sourceDate")
	sourceDate := time.Now()
	if sourceDateStr != "" {
		if d, err := time.Parse(time.RFC3339, sourceDateStr); err == nil {
			sourceDate = d
		} else {
			fmt.Println(err.Error())
		}
	}
	fmt.Println(sourceId, " ", sourceDateStr)

	photo := dao.Photo{}
	photo.Id = uuid.New()
	photo.Source = dao.SourceLocal
	photo.SourceId = sourceId
	photo.Md5 = md5str
	photo.SourceDate = sourceDate
	photo.UploadDate = time.Now()
	photo.FileName = photo.Id.String() + ".jpg"
	photo.Private = true

	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	dst, err := os.Create(config.PhotoFilePath(config.Original, photo.FileName))
	if _, err = io.Copy(dst, file); err != nil {
		return nil, err
	}
	dst.Close()

	/*
		if err := GenerateImages(config.PhotoFilePath(config.Original, photo.FileName), config.ServiceRoot()); err != nil {
			return nil, err
		}*/
	if err := dao.GenerateImages(photo.FileName); err != nil {
		return nil, err
	}

	var md *metadata.MetaData
	if md, err = metadata.NewMetaDataFromFile(config.PhotoFilePath(config.Original, photo.FileName)); err == nil {
		photo.CameraMake = md.Summary().CameraMake
		photo.CameraModel = md.Summary().CameraModel
		photo.FocalLength = fmt.Sprintf("%v mm", md.Summary().FocalLength.Float32())
		photo.FocalLength35 = fmt.Sprintf("%v mm", md.Summary().FocalLengthIn35mmFormat)
		photo.LensMake = md.Summary().LensMake
		photo.LensModel = md.Summary().LensModel
		photo.Exposure = md.Summary().ExposureTime.String()
		photo.Width = md.ImageWidth
		photo.Height = md.ImageHeight
		photo.FNumber = md.Summary().FNumber.Float32()
		photo.Iso = uint(md.Summary().ISO)
		photo.Title = md.Summary().Title
		if len(md.Summary().Keywords) > 0 {
			photo.Keywords = strings.Join(md.Summary().Keywords, ",")
		}
		if md.Summary().OriginalDate.IsZero() {
			photo.OriginalDate = photo.SourceDate
		} else {
			photo.OriginalDate = md.Summary().OriginalDate
		}
	} else {
		return nil, err
	}

	if err = s.pg.Photo.Add(&photo, md.Summary()); err != nil {
		s.l.Errorw("error adding img: ", zap.Error(err))
		return nil, err
	}
	if !s.pg.Camera.HasModel(photo.CameraModel) {
		if err = s.pg.Camera.AddFromPhoto(&photo); err != nil {
			s.l.Fatalw("error adding camera model: ", zap.Error(err))
		}
	}

	s.l.Infow("added img", "Id", photo.Id, "SourceId", photo.SourceId)

	return &photo, nil
}

/*
func (s *mserver) handleUploadLocalPhoto(r *http.Request) (interface{}, error) {

	r.ParseMultipartForm(10 << 20) //10M
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

	//Detect type
	buff := make([]byte, 512)
	_, err = file.Read(buff)
	if err != nil {
		return nil, err
	}
	mt := http.DetectContentType(buff)
	if mt != gdrive.Jpeg {
		return nil, BadRequestError("Image is not of the correct mimetype: " + mt)
	}

	sourceId := r.FormValue("sourceId")
	if sourceId == "" {
		sourceId = head.Filename
	}
	sourceDateStr := r.FormValue("sourceDate")
	sourceDate := time.Now()
	if sourceDateStr != "" {
		if d,err := time.Parse(time.RFC3339, sourceDateStr); err == nil {
			sourceDate = d
		} else {
			fmt.Println(err.Error())
		}
	}
	fmt.Println(sourceId, " ", sourceDateStr)

	photo := dao.Photo{}
	photo.Id = uuid.New()
	photo.Source = dao.SourceLocal
	photo.SourceId = sourceId
	photo.Md5 = md5str
	photo.SourceDate = sourceDate
	photo.UploadDate = time.Now()
	photo.FileName = photo.Id.String()+".jpg"
	photo.Private = true

	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	dst, err := os.Create(imgPath(s, photo.FileName))
	if _, err = io.Copy(dst, file); err != nil {
		return nil, err
	}
	dst.Close()

	if err := GenerateImages(imgPath(s, photo.FileName), config.ServiceRoot()); err != nil {
		return nil, err
	}

	var exif *mexif.ExifCompact
	tool, err := mexif.NewMExifTool()
	if err != nil {
		return nil, err
	}

	if exif, err = tool.ExifCompact(imgPath(s, photo.FileName)); err == nil {
		photo.CameraMake = exif.CameraMake
		photo.CameraModel = exif.CameraModel
		photo.FocalLength = exif.FocalLength
		photo.FocalLength35 = exif.FocalLengthIn35mmFormat
		photo.LensMake = exif.LensMake
		photo.LensModel = exif.LensModel
		photo.Exposure = exif.ExposureTime
		photo.Width = exif.ImageWidth
		photo.Height = exif.ImageHeight
		photo.FNumber = exif.FNumber
		photo.Iso = exif.ISO
		photo.Title = exif.Title
		if len(exif.Keywords) > 0 {
			photo.Keywords = strings.Join(exif.Keywords, ",")
		}
		if exif.OriginalDate.IsZero() {
			photo.OriginalDate = photo.SourceDate
		} else {
			photo.OriginalDate = exif.OriginalDate
		}
	} else {
		return nil, err
	}

	if err = s.pg.Photo.Add(&photo, exif); err != nil {
		s.l.Errorw("error adding img: ", zap.Error(err))
		return nil, err
	}
	if !s.pg.Camera.HasModel(photo.CameraModel) {
		if err = s.pg.Camera.AddFromPhoto(&photo); err != nil {
			s.l.Fatalw("error adding camera model: ", zap.Error(err))
		}
	}

	s.l.Infow("added img", "Id", photo.Id, "SourceId", photo.SourceId)

	return &photo, nil

}
*/
func (s *mserver) handleCheckLocalPhotos(r *http.Request) (interface{}, error) {

	return nil, nil
}
