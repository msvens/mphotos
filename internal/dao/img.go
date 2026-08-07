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

func DeleteImg(fname string) error {
	for pt := range config.PhotoPaths() {
		fpath := config.PhotoFilePath(pt, fname)
		err := os.Remove(fpath)
		if err != nil && !os.IsNotExist(err) {
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
	fNames := make(map[string]bool)
	for _, p := range photos {
		fNames[p.FileName] = true
	}
	for k := range config.PhotoPaths() {
		err := cleanImgDir(fNames, k)
		if err != nil {
			logger.Errorw("Error cleaning imgDir", "error", err)
		}
	}
	return nil
}

func GenerateImages(fName string) error {
	srcFile := config.PhotoFilePath(config.Original, fName)
	// TransformFile derives each variant's extension from its Options.Format, so the
	// destination keys must be base paths without an extension (<uuid>, not <uuid>.jpg).
	base := strings.TrimSuffix(fName, filepath.Ext(fName))
	imgMap := map[string]img.Options{}
	for pt, opt := range photoTypes {
		imgMap[config.PhotoFilePath(pt, base)] = opt
	}
	return img.TransformFile(srcFile, imgMap)
}
