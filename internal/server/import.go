package server

import (
	"fmt"
	"os"
	"strings"

	"github.com/msvens/mimage/img"
	"github.com/msvens/mimage/metadata"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"go.uber.org/zap"
)

// finalizeImport turns a staged source file into the stored jpeg original,
// extracts metadata + dimensions, applies the "No Camera" sentinel when no
// camera EXIF is present, generates the display variants, and persists the photo
// and its camera. srcFormat is the source's already-detected format.
//
// The conversion is delegated to mimage's ConvertFile: it is a no-op
// (converted == false) when the source is already jpeg — in which case the
// staged file is moved into place as <uuid>.jpg — and otherwise re-encodes to
// <uuid>.jpg, carrying EXIF/XMP/IPTC across (e.g. tiff -> jpeg).
func (s *mserver) finalizeImport(photo *dao.Photo, stagedPath string, srcFormat img.Format) error {
	base := config.PhotoFilePath(config.Original, photo.Id.String()) // no extension; ConvertFile appends it
	jpgPath := config.PhotoFilePath(config.Original, photo.FileName) // <uuid>.jpg

	if _, converted, err := img.ConvertFile(stagedPath, base, img.FormatJpeg, img.NewConvertOptions(true)); err != nil {
		return err
	} else if converted {
		_ = os.Remove(stagedPath) // non-jpeg source: ConvertFile wrote <uuid>.jpg, drop the staged source
	} else if err := os.Rename(stagedPath, jpgPath); err != nil {
		return err // jpeg source: keep as-is, just move it into place
	}

	// Metadata + dimensions from the stored jpeg. Uniform across formats: a jpeg
	// with no EXIF (png/gif/bmp-derived, or a stripped jpeg) parses to an empty
	// Summary without error, and ImageWidth/Height come from the pixel decode.
	md, err := metadata.NewMetaDataFromFile(jpgPath)
	if err != nil {
		return err
	}
	summary := md.Summary()
	applySummary(photo, summary)
	photo.Width = md.ImageWidth
	photo.Height = md.ImageHeight
	if photo.OriginalDate.IsZero() {
		photo.OriginalDate = photo.SourceDate
	}

	photo.SourceOther = srcFormat.String()
	if photo.CameraModel == "" {
		photo.CameraModel = dao.NoCameraModel
	}

	if err := dao.GenerateImages(photo.FileName); err != nil {
		return err
	}
	if err := s.pg.Photo.Add(photo, summary); err != nil {
		s.l.Errorw("error adding img", zap.Error(err))
		return err
	}
	if !s.pg.Camera.HasModel(photo.CameraModel) {
		if err := s.pg.Camera.AddFromPhoto(photo); err != nil {
			s.l.Errorw("error adding camera model", zap.Error(err))
			return err
		}
	}
	return nil
}

// variantFormat is the mimage output format for the size variants of a stored
// camera/avatar image, which keeps its source format (jpeg or png) rather than
// being normalized to jpeg like photos.
func variantFormat(ext string) img.Format {
	if ext == ".png" {
		return img.FormatPng
	}
	return img.FormatJpeg
}

// applySummary copies EXIF-derived fields from a Summary onto the photo.
func applySummary(photo *dao.Photo, sum *metadata.Summary) {
	photo.CameraMake = sum.CameraMake
	photo.CameraModel = sum.CameraModel
	photo.FocalLength = fmt.Sprintf("%v mm", sum.FocalLength.Float32())
	photo.FocalLength35 = fmt.Sprintf("%v mm", sum.FocalLengthIn35mmFormat)
	photo.LensMake = sum.LensMake
	photo.LensModel = sum.LensModel
	photo.Exposure = sum.ExposureTime.String()
	photo.FNumber = sum.FNumber.Float32()
	photo.Iso = uint(sum.ISO)
	photo.Title = sum.Title
	if len(sum.Keywords) > 0 {
		photo.Keywords = strings.Join(sum.Keywords, ",")
	}
	if !sum.OriginalDate.IsZero() {
		photo.OriginalDate = sum.OriginalDate
	}
}
