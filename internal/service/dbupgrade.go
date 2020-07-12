package service

import "fmt"

func (dbs *DbService) UpgradeDb() error {
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
		upgradeToV2(dbs)
	} else {
		fmt.Println("Database already on Version 2")
	}
	return nil
}

const addColumnsPhotosV2 = `
ALTER TABLE photos
	ADD COLUMN private BOOLEAN,
	ADD COLUMN album TEXT,
	ADD COLUMN likes INTEGER;
`
const setColumnsPhotosV2 = `UPDATE photos SET private = false, album = "", likes = 0;`

const setConstraintsPhotosV2 = `
ALTER TABLE photos
	ALTER COLUMN private SET NOT NULL,
	ALTER COLUMN album SET NOT NULL,
	ALTER COLUMN likes SET NOT NULL;
`

func upgradeToV2(dbs *DbService) error {
	fmt.Println("Upgrading mphotos db to Version 2")
	_, err := dbs.Db.Exec(addColumnsPhotosV2)
	if err != nil {
		return err
	}
	_, err = dbs.Db.Exec(setColumnsPhotosV2)
	if err != nil {
		return err
	}
	_, err = dbs.Db.Exec(setConstraintsPhotosV2)
	return err

}
