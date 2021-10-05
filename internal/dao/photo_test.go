package dao

import (
	"encoding/json"
	"github.com/msvens/mexif"
	"io/ioutil"
	"strconv"
)

var testPhotos []Photo
var testExifs []mexif.ExifCompact

var loadedTestData = false

func loadPhotoTestData() error {
	if loadedTestData {
		return nil
	}
	photoData, err := ioutil.ReadFile("../../testphoto.json")
	if err != nil {
		return err
	}
	err = json.Unmarshal(photoData, &testPhotos)
	//add dummy sourceId
	for idx, p := range testPhotos {
		p.SourceId = "sourceId" + strconv.Itoa(idx)
	}
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
