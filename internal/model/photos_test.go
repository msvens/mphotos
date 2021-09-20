package model

import (
	"encoding/json"
	"github.com/msvens/mexif"
	"io/ioutil"
)

var testPhotos []Photo
var testExifs []mexif.ExifCompact

var loadedTestData = false

func loadPhotoTestData() error {
	if loadedTestData {
		return nil
	}
	photoData, err := ioutil.ReadFile("../../testphotos.json")
	if err != nil {
		return err
	}
	err = json.Unmarshal(photoData, &testPhotos)
	if err != nil {
		return err
	}

	exifData, err := ioutil.ReadFile("../../testexif.json")
	err = json.Unmarshal(exifData, &testExifs)
	if err != nil {
		return err
	}

	loadedTestData = true
	return nil
}
