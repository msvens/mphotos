package service

import (
	"fmt"
)

func (dbs *DbService) UpgradeDb() error {

	//Check for version 1:
	rows, err := dbs.Db.Query("SELECT * from photos limit 1")
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
		return upgradeToV4FromV1(dbs)
	}

	//Check for version 2:
	if _, err := dbs.Db.Query(getAlbumsStmt); err != nil {
		return upgradeToV3FromV2(dbs)
	}

	//Check for version 3:
	if _, err := dbs.Db.Query("SELECT * from albumphoto"); err != nil {
		return upgradeToV4FromV3(dbs)
	}

	fmt.Println("Databes looks to be up to date")
	return nil

	//Check for version 3:

}

func upgradeToV4FromV3(dbs *DbService) error {
	photoAlbumQ := "SELECT driveId,album from photos WHERE album != ''"
	dropAlbumColumn := "ALTER TABLE photos DROP COLUMN album"

	fmt.Println("Upgrading mphotos db from Version 3 to Version 4")
	fmt.Println("Adding albumphoto table")
	if _, err := dbs.Db.Exec(createAlbumPhotoTable); err != nil {
		return err
	}
	//move album/photo information to albumphoto
	fmt.Println("Adding album-photo mappings to albumphoto")
	if rows, err := dbs.Db.Query(photoAlbumQ); err != nil {
		return err
	} else {
		for rows.Next() {
			var id string
			var a string
			rows.Scan(&id, &a)
			albums := trimAndSplit(a)
			dbs.UpdatePhotoAlbum(albums, id)
		}
	}
	//drop album column
	fmt.Println("Drop album column from photos")
	if _, err := dbs.Db.Exec(dropAlbumColumn); err != nil {
		return err
	}
	return nil
}

func upgradeToV3FromV2(dbs *DbService) error {
	fmt.Println("Upgrading mphotos db from Version 2 to Version 3")
	fmt.Println("Adding Album Table")
	if _, err := dbs.Db.Exec(createAlbumTable); err != nil {
		return err
	}

	fmt.Println("Adding Albums from photos")
	if rows, err := dbs.Db.Query(distinctAlbumsStmt); err != nil {
		return err
	} else {
		for rows.Next() {
			var a string
			rows.Scan(&a)
			albums := trimAndSplit(a)
			for _, album := range albums {
				if album != "" && !dbs.ContainsAlbum(album) {
					fmt.Println("adding album: ", album)
					err := dbs.AddAlbum(&Album{Name: album})
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

//TODO: Update to V4 (and not V3)

func upgradeToV4FromV1(dbs *DbService) error {
	addColumnsPhotosV2 := `
ALTER TABLE photos
	ADD COLUMN private BOOLEAN
	ADD COLUMN likes INTEGER;
`
	setColumnsPhotosV2 := `UPDATE photos SET private = false, likes = 0;`

	setConstraintsPhotosV2 := `
ALTER TABLE photos
	ALTER COLUMN private SET NOT NULL,
	ALTER COLUMN likes SET NOT NULL;
`

	fmt.Println("Upgrading mphotos db from Version 1 to Version 3")
	fmt.Println("Adding columns to photos")
	_, err := dbs.Db.Exec(addColumnsPhotosV2)
	if err != nil {
		return err
	}
	fmt.Println("Set column values")
	_, err = dbs.Db.Exec(setColumnsPhotosV2)
	if err != nil {
		return err
	}
	fmt.Println("Upgrading mphotos db to Version 2")
	fmt.Println("Sec column constraint not null")
	_, err = dbs.Db.Exec(setConstraintsPhotosV2)

	if err != nil {
		return err
	}

	fmt.Println("Create Album Table")
	_, err = dbs.Db.Exec(createAlbumTable)
	if err != nil {
		return err
	}

	fmt.Println("Create AlbumPhoto Table")
	_, err = dbs.Db.Exec(createAlbumPhotoTable)
	return err
}
