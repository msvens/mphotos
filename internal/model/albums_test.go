package model

import (
	"fmt"
	"testing"
)

func cmpAlbum(exp, act Album, id bool) error {
	if id && exp != act {
		return fmt.Errorf("Expected %s got %s", exp, act)
	} else if exp.Name != act.Name || exp.Description != act.Description || exp.CoverPic != act.CoverPic {
		return fmt.Errorf("Expected same (name, description and coverpic) %s got %s", exp, act)
	} else {
		return nil
	}
}

func TestAlbums(t *testing.T) {
	var err error
	ds := openAndCreateTestDb(t)

	first := Album{Name: "album1", Description: "description1", CoverPic: "coverpic1"}
	second := Album{Name: "album2", Description: "description2", CoverPic: "coverpic2"}

	//add album: Three cases. 1. Add album 2. Add Another album with the same name 3. Add album with empty name
	if act, err := ds.AddAlbum(first.Name, first.Description, first.CoverPic); err != nil {
		t.Errorf("Could not add album. Got error: %s", err)
	} else {
		first.Id = act.Id
		if e1 := cmpAlbum(first, *act, true); e1 != nil {
			t.Error(e1)
		}
	}
	if _, err = ds.AddAlbum(first.Name, first.Description, first.CoverPic); err == nil {
		t.Error("Expected error when trying to add album with the same name")
	}
	if _, err = ds.AddAlbum("", "", ""); err == nil {
		t.Error("Expected error when adding an album with an empty name")
	}
	//Album existence:
	if !ds.HasAlbum(first.Id) {
		t.Error("Expected to find album: ", first.Id)
	}
	if !ds.HasAlbumName(first.Name) {
		t.Error("Expected to find album: ", first.Name)
	}
	if ds.HasAlbum(-1) {
		t.Error("No album with Id -1 should exist")
	}
	if ds.HasAlbumName("noname") {
		t.Error("No album with name noname should exist")
	}
	//Retrieving Albums. Add an additional album for testing slices
	a2, _ := ds.AddAlbum(second.Name, second.Description, second.CoverPic)
	second.Id = a2.Id

	if act, err := ds.Album(first.Id); err != nil {
		t.Error("Expected to get album got error: ", err)
	} else if e1 := cmpAlbum(first, *act, true); e1 != nil {
		t.Error(e1)
	}

	if act, err := ds.Album(-1); err == nil {
		t.Error("Expected error got album: ", act)
	}

	if albums, err := ds.Albums(); err != nil {
		t.Error("got error when retrieving albums: ", err)
	} else if len(albums) != 2 {
		t.Error("expected to get 2 errors got ", len(albums))
	} else {
		a1 := albums[0]
		a2 := albums[1]
		if a1.Id != first.Id {
			a1 = albums[1]
			a2 = albums[0]
		}
		if cmpAlbum(first, *a1, true) != nil || cmpAlbum(second, *a2, true) != nil {
			t.Error("albums did not retrieve expected albums")
		}
	}

	//test update albums
	updatedFirst := first
	updatedFirst.Description = "new description"
	if ret, err := ds.UpdateAlbum(&updatedFirst); err != nil {
		t.Error("albums could not be updated ", err)
	} else if *ret != updatedFirst {
		t.Errorf("expected %s got %s", updatedFirst, *ret)
	}

	//expect failure if you try to update an album to an already existing name
	updatedFirst.Name = second.Name
	if _, err = ds.UpdateAlbum(&updatedFirst); err == nil {
		t.Error("expected error when updating an album to an existing name")
	}

	//Tests to add, get album photos and photos in albums
	err = loadPhotoTestData()
	if err != nil {
		t.Errorf("Could not load photo test data")
	}
	err = ds.AddPhoto(&testPhotos[0], &testExifs[0])
	if err != nil {
		t.Error("Could not create photo: ", err)
	}

	err = ds.UpdatePhotoAlbums([]int{updatedFirst.Id, second.Id}, testPhotos[0].DriveId)
	if err != nil {
		t.Error("could not update photos albums: ", err)
	}

	err = ds.UpdatePhotoAlbums([]int{updatedFirst.Id, second.Id}, "nonexistent")

	if err == nil {
		t.Error("Expected error when adding a non-existent photo to an album")
	}

	if err = ds.UpdatePhotoAlbums([]int{0}, testPhotos[0].DriveId); err == nil {
		t.Error("Expected error when adding a photo to a non existent album")
	}

	albums, err := ds.PhotoAlbums(testPhotos[0].DriveId)
	if err != nil {
		t.Error("could not retrieve photo albums ", err)
	} else {
		if len(albums) != 2 {
			t.Error("Expected 2 albums got: ", len(albums))
		}
		if albums[0].Id != updatedFirst.Id && albums[1].Id != updatedFirst.Id {
			t.Errorf("Did not get %v in album list", updatedFirst.Id)
		}
		if albums[0].Id != second.Id && albums[1].Id != second.Id {
			t.Errorf("Did not get %v in album list", second.Id)
		}
	}

	if photos, err := ds.AlbumPhotos(updatedFirst.Id, PhotoFilter{}); err != nil {
		t.Error("Could not get album photos got error: ", err)
	} else {
		if len(photos) != 1 {
			t.Error("Expected 1 photo album got ", len(photos))
		}
		if photos[0].DriveId != testPhotos[0].DriveId {
			t.Errorf("Expected photoId %v got %v", testPhotos[0].DriveId, photos[0].DriveId)
		}
	}

	deleteAndCloseTestDb(ds, t)

}
