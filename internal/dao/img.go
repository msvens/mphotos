package dao

import (
	"github.com/msvens/mimage/img"
	"github.com/msvens/mphotos/internal/config"
	"os"
	"path/filepath"
	"strings"
)

// Display variants are always jpeg (the stored original is always jpeg).
var (
	thumb     = img.NewOptions(img.ResizeAndCrop, 400, 400, false, img.FormatJpeg)
	landscape = img.NewOptions(img.ResizeAndCrop, 1200, 628, true, img.FormatJpeg)
	square    = img.NewOptions(img.ResizeAndCrop, 1200, 1200, true, img.FormatJpeg)
	portrait  = img.NewOptions(img.ResizeAndCrop, 1080, 1350, true, img.FormatJpeg)
	resize    = img.NewOptions(img.Resize, 1200, 0, true, img.FormatJpeg)
)

var photoTypes = map[config.PhotoType]img.Options{
	config.Thumb:     thumb,
	config.Landscape: landscape,
	config.Square:    square,
	config.Portrait:  portrait,
	config.Resize:    resize,
}

func CreateImageDirs() error {
	for _, path := range config.PhotoPaths() {
		if err := os.MkdirAll(path, 0744); err != nil {
			return err
		}
	}
	return nil
}

//Removes any images that are not in the db

// DeleteImg removes a media item's stored files: the original by its filename
// (e.g. <uuid>.jpg or <uuid>.mp4) and the size variants, which are always
// <base>.jpg regardless of the original's kind. For photos the original and the
// variant name coincide; for videos they differ (.mp4 vs .jpg).
func DeleteImg(fname string) error {
	base := strings.TrimSuffix(fname, filepath.Ext(fname))
	for pt := range config.PhotoPaths() {
		name := fname
		if pt != config.Original {
			name = base + ".jpg"
		}
		fpath := config.PhotoFilePath(pt, name)
		if err := os.Remove(fpath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func cleanImgDir(keep map[string]bool, pt config.PhotoType) error {
	files, err := os.ReadDir(config.PhotoPath(pt))
	if err != nil {
		return err
	}
	logger.Infow("Cleaning", "path", config.PhotoPath(pt))
	numDeleted := 0
	for _, f := range files {
		if !keep[f.Name()] {
			fpath := config.PhotoFilePath(pt, f.Name())
			err = os.Remove(fpath)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			numDeleted++
		}
	}
	logger.Infow("Deleted files", "count", numDeleted, "path", config.PhotoPath(pt))
	return nil
}

func CleanImageDirs(db *PGDB) error {
	photos, err := db.Photo.List()
	if err != nil {
		return err
	}
	// The Original dir keeps each item's real filename (<uuid>.jpg or <uuid>.mp4);
	// the variant dirs keep <base>.jpg (variants are always jpeg, incl. video posters).
	keepOriginal := make(map[string]bool)
	keepVariant := make(map[string]bool)
	for _, p := range photos {
		keepOriginal[p.FileName] = true
		base := strings.TrimSuffix(p.FileName, filepath.Ext(p.FileName))
		keepVariant[base+".jpg"] = true
	}
	for pt := range config.PhotoPaths() {
		keep := keepVariant
		if pt == config.Original {
			keep = keepOriginal
		}
		if err := cleanImgDir(keep, pt); err != nil {
			logger.Errorw("Error cleaning imgDir", "error", err)
		}
	}
	return nil
}

// GenerateImages generates the size variants for a stored original (a jpeg photo),
// deriving the source path from its filename.
func GenerateImages(fName string) error {
	srcFile := config.PhotoFilePath(config.Original, fName)
	base := strings.TrimSuffix(fName, filepath.Ext(fName))
	return GenerateImagesFromSource(srcFile, base)
}

// GenerateImagesFromSource generates the size variants from an explicit source
// image into <variant>/<baseName>.jpg for each variant. Used for photos (source =
// the stored original) and for videos (source = the extracted poster jpeg).
// TransformFile derives each variant's extension from its Options.Format, so the
// destination keys must be base paths without an extension (<uuid>, not <uuid>.jpg).
func GenerateImagesFromSource(srcImagePath, baseName string) error {
	imgMap := map[string]img.Options{}
	for pt, opt := range photoTypes {
		imgMap[config.PhotoFilePath(pt, baseName)] = opt
	}
	return img.TransformFile(srcImagePath, imgMap)
}
