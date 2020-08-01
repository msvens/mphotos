package model

import "fmt"

func UpgradeDb(db *DB) error {

	//Check for version 1:
	rows, err := db.Query("SELECT * from photos limit 1")
	if err != nil {
		return err
	}
	colNames, err := rows.Columns()
	if err != nil {
		return err
	}
	var hasPrivate bool
	//Version 2 of the Db added private, stars and albums to the photos table
	for _, col := range colNames {
		if col == "private" {
			hasPrivate = true
		}
	}

	if !hasPrivate {
		return upgradeToV4FromV1(db)
	}

	//Check for version 3:
	if _, err := db.Query("SELECT * from albumphoto"); err != nil {
		return upgradeToV4FromV3(db)
	}

	fmt.Println("Databes looks to be up to date")
	return nil

	//Check for version 3:

}

func upgradeToV4FromV3(db *DB) error {
	photoAlbumQ := "SELECT driveId,album from photos WHERE album != ''"
	dropAlbumColumn := "ALTER TABLE photos DROP COLUMN album"

	fmt.Println("Upgrading mphotos db from Version 3 to Version 4")
	fmt.Println("Adding albumphoto table")
	if err := db.CreateAlbumPhotoStore(); err != nil {
		return err
	}
	//move album/photo information to albumphoto
	fmt.Println("Adding album-photo mappings to albumphoto")
	if rows, err := db.Query(photoAlbumQ); err != nil {
		return err
	} else {
		for rows.Next() {
			var id string
			var a string
			rows.Scan(&id, &a)
			albums := trimAndSplit(a)
			db.UpdatePhotoAlbums(albums, id)
		}
	}
	//drop album column
	fmt.Println("Drop album column from photos")
	if _, err := db.Exec(dropAlbumColumn); err != nil {
		return err
	}
	return nil
}

func upgradeToV4FromV1(db *DB) error {
	addColumnsPhotosV2 := `
ALTER TABLE photos
	ADD COLUMN private BOOLEAN,
	ADD COLUMN likes INTEGER;
`
	setColumnsPhotosV2 := `UPDATE photos SET private = false, likes = 0;`

	setConstraintsPhotosV2 := `
ALTER TABLE photos
	ALTER COLUMN private SET NOT NULL,
	ALTER COLUMN likes SET NOT NULL;
`

	fmt.Println("Upgrading mphotos db from Version 1 to Version 4")
	fmt.Println("Adding columns to photos")
	_, err := db.Exec(addColumnsPhotosV2)
	if err != nil {
		return err
	}
	fmt.Println("Set column values")
	_, err = db.Exec(setColumnsPhotosV2)
	if err != nil {
		return err
	}
	fmt.Println("Upgrading mphotos db to Version 2")
	fmt.Println("Sec column constraint not null")
	_, err = db.Exec(setConstraintsPhotosV2)

	if err != nil {
		return err
	}

	fmt.Println("Create Album Table")
	err = db.CreateAlbumStore()
	if err != nil {
		return err
	}

	fmt.Println("Create AlbumPhoto Table")
	err = db.CreateAlbumPhotoStore()
	return err
}
