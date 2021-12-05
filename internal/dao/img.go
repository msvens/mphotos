package dao

import (
	"fmt"
	"github.com/msvens/mimage/img"
	"github.com/msvens/mphotos/internal/config"
	"os"
)

var (
	thumb     = img.NewOptions(img.ResizeAndCrop, 400, 400, false)
	landscape = img.NewOptions(img.ResizeAndCrop, 1200, 628, true)
	square    = img.NewOptions(img.ResizeAndCrop, 1200, 1200, true)
	portrait  = img.NewOptions(img.ResizeAndCrop, 1080, 1350, true)
	resize    = img.NewOptions(img.Resize, 1200, 0, true)
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
	for pt, _ := range config.PhotoPaths() {
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
	fmt.Printf("Cleaning: %s\n", config.PhotoPath(pt))
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
	fmt.Printf("Deleted %v files from %v\n", numDeleted, config.PhotoPath(pt))
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
	for k, _ := range config.PhotoPaths() {
		err := cleanImgDir(fNames, k)
		if err != nil {
			fmt.Println("Error cleaning imgDir: ", err)
		}
	}
	return nil

	//list image dir:
	/*files, err := ioutil.ReadDir(config.PhotoPath(config.Original))
	if err != nil {
		return nil
	}
	toDelete := []string{}
	for _, f := range files {
		if !fNames[f.Name()] {
			toDelete = append(toDelete, f.Name())
		}
	}
	for _, td := range toDelete {
		fmt.Println("Deleteing: ", td)
		DeleteImg(td)
	}
	return nil
	*/
}

/*
func CreateImageDir(dir string) error {
	var err error
	if err = os.MkdirAll(dir, 0744); err != nil {
		return err
	}
	for name, _ := range photoTypes {
		if err = os.MkdirAll(filepath.Join(dir, name), 0744); err != nil {
			return err
		}
	}
	return nil
}

*/

func GenerateImages(fName string) error {
	srcFile := config.PhotoFilePath(config.Original, fName)
	//base := filepath.Base(srcFile)
	imgMap := map[string]img.Options{}

	for pt, opt := range photoTypes {
		imgMap[config.PhotoFilePath(pt, fName)] = opt
	}
	return img.TransformFile(srcFile, imgMap)
	/*
		base := filepath.Base(srcFile)

		imgMap := map[string]img.Options{}
		for name, opt := range photoTypes {
			imgMap[filepath.Join(dir, name, base)] = opt
		}
		return img.TransformFile(srcFile, imgMap)
	*/

}
