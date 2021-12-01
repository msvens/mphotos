package dao

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
)

var testPhotos []Photo
var testExifs []Exif

var loadedTestData = false

func loadPhotoTestData() error {
	if loadedTestData {
		return nil
	}
	photoData, err := ioutil.ReadFile("../../assets/photos.json")
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

	fmt.Println("Now read exif data")
	exifData, err := ioutil.ReadFile("../../assets/exif.json")
	err = json.Unmarshal(exifData, &testExifs)
	if err != nil {
		return err
	}

	fmt.Println("exif data unmarshalled")
	loadedTestData = true
	return nil
}
