package model

import "fmt"

func UpgradeDb(db *DB) error {

	/*
		DB CHANGES
		VERSION 5: Added UserConfig
		VERSION 4: Added AlbumPhoto table
		VERSION 3: Added Album table
	*/
	return upgradeToV5(db)

}

func upgradeToV5(db *DB) error {
	rows, err := db.Query("SELECT * from users LIMIT 1")
	if err != nil {
		return err
	}

	colNames, err := rows.Columns()
	if err != nil {
		return err
	}
	var hasConfig bool
	for _, col := range colNames {
		if col == "config" {
			hasConfig = true
		}
	}
	if !hasConfig {
		fmt.Println("Upgrading mphotos db from Version 4 to Version 5")
		addColumnStmt := "ALTER TABLE users ADD COLUMN config TEXT;"
		setColumnStmt := "UPDATE users SET config = '{}';"
		conColumnStmt := "ALTER TABLE users ALTER COLUMN config SET NOT NULL;"

		fmt.Println("Adding column to users")
		_, err := db.Exec(addColumnStmt)
		if err != nil {
			return err
		}
		fmt.Println("Set column value")
		_, err = db.Exec(setColumnStmt)
		if err != nil {
			return err
		}

		fmt.Println("Sec column constraint not null")
		_, err = db.Exec(conColumnStmt)

		if err != nil {
			return err
		}
		fmt.Println("Database Upgraded to version 5")
		return nil
	} else {
		fmt.Println("Databes looks to be up to date")
		return nil
	}
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
