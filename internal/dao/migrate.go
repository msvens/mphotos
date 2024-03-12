package dao

import (
	"fmt"
)

// for now this is just hard coded
func canUpgradeDb(pgdb *PGDB) bool {
	if v, err := pgdb.Version.Get(); err != nil {
		fmt.Println("could not get Version info: ", err)
		return false
	} else {
		return v.VersionId+1 == DbVersion
	}
}

func UpgradeDb() error {
	var err error
	var pgdb *PGDB
	if pgdb, err = NewPGDB(); err != nil {
		return err
	}
	hasVersion := pgdb.tableExists("version")
	hasPhotos := pgdb.tableExists("photos")
	if hasVersion {
		if isCurrent, err := pgdb.Version.IsCurrent(); err != nil {
			return err
		} else if isCurrent {
			fmt.Println("Database is up to date")
			return nil
		} else if canUpgradeDb(pgdb) {
			return upgradeToV3(pgdb)
		} else {
			return fmt.Errorf("Cannot upgrade database, wrong current version")
		}
	} else if hasPhotos {
		return fmt.Errorf("Cannot upgrade database")
	} else {
		fmt.Println("No database exists. Creating a fresh db")
		if err = pgdb.CreateTables(); err != nil {
			return err
		}
	}
	return nil
}

func upgradeToV3(pgdb *PGDB) error {
	fmt.Println("Upgrading Db to Version: ", DbVersion)

	var err error
	var v *Version
	var a *Album
	var photos []*Photo

	if _, err = pgdb.db.Exec(schemaV2toV3); err != nil {
		return err
	}

	fmt.Println("Db Updated. Change Version Info")
	if v, err = pgdb.Version.Update(); err != nil {
		return err
	} else {
		fmt.Println("Updated Db to version: ", v.VersionId)
	}
	fmt.Println("create photo stream album")
	if a, err = pgdb.Album.Add("photostream", "Default public photostream", ""); err != nil {
		return err
	}

	fmt.Println("Add all public photos to it")

	if err = pgdb.db.Select(&photos, "SELECT * FROM img WHERE private = false"); err != nil {
		return err
	}

	const addAlbumPhoto = "INSERT INTO albumphotos (albumId, photoId) VALUES ($1, $2)"
	for _, p := range photos {
		if _, err := pgdb.db.Exec(addAlbumPhoto, a.Id, p.Id); err != nil {
			return nil
		}
	}

	//finally delete the private column
	fmt.Println("Drop the private column from image")
	_, err = pgdb.db.Exec("ALTER TABLE img DROP COLUMN private")
	return err
}

func upgradeToV2(pgdb *PGDB) error {
	fmt.Println("Upgrading Db to Version: ", DbVersion)
	var err error
	_, err = pgdb.db.Exec(schemaV1toV2)
	if err != nil {
		return err
	}
	fmt.Println("Db Updated. Change Version Info")
	if v, err := pgdb.Version.Update(); err != nil {
		return err
	} else {
		fmt.Println("Updated Db to version: ", v.VersionId)
	}
	return nil
}
