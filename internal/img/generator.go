package img

import (
	"github.com/h2non/bimg"
	"os"
	"path/filepath"
)

func option(width, height int) bimg.Options {
	return bimg.Options{Quality: 90, Crop: true, Gravity: bimg.GravityCentre, Width: width, Height: height}
}

var (
	thumb     = option(400, 400)
	landscape = option(1200, 628)
	square    = option(1200, 1200)
	portraite = option(1080, 1350)
	resize    = bimg.Options{Quality: 90, Width: 1200}

	imageTypes = map[string]bimg.Options{"thumb": thumb, "landscape": landscape,
		"square": square, "portrait": portraite, "resize": resize}
)

func CreateImageDir(dir string) error {
	var err error
	if err = os.MkdirAll(dir, 0744); err != nil {
		return err
	}
	for name, _ := range imageTypes {
		if err = os.MkdirAll(filepath.Join(dir, name), 0744); err != nil {
			return err
		}
	}
	return nil
}

func GenerateImages(srcFile, dir string) error {
	buffer, err := bimg.Read(srcFile)

	if err != nil {
		return err
	}
	base := filepath.Base(srcFile)
	img := bimg.NewImage(buffer)

	for name, options := range imageTypes {
		if buff, err := bimg.Resize(img.Image(), options); err != nil {
			return err
		} else {
			bimg.Write(filepath.Join(dir, name, base), buff)
		}
	}
	return nil
}
