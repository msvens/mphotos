package model

import "fmt"

func UpgradeDb(db *DB) error {

	/*
		DB CHANGES
		VERSION 6: Added serial as primary key for Albums (not names)
		VERSION 5: Added UserConfig
		VERSION 4: Added AlbumPhoto table
		VERSION 3: Added Album table
	*/
	return upgradeToV6(db)
}

func upgradeToV6(db *DB) error {
	rows, err := db.Query("SELECT column_name FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = 'albums'")
	if err != nil {
		return err
	}
	hasId := false
	for rows.Next() {
		var colName string
		rows.Scan(&colName)
		if colName == "id" {
			hasId = true
		}
	}
	if hasId {
		fmt.Println("Already at Version 6")
		return nil
	}

	fmt.Println("Dropping old albums and albumphoto tables")
	const stmt = `
	DROP TABLE IF EXISTS albumphoto;
	DROP TABLE IF EXISTS albums;
`
	_, err = db.Exec(stmt)
	if err != nil {
		return err
	}
	fmt.Println("Creating new albums and albumphoto tables")
	db.CreateAlbumStore()
	return err
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
