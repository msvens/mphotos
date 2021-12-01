package dao

import (
	"github.com/msvens/mphotos/internal/config"
	"testing"
)

func openAndCreateTestDb(t *testing.T) *PGDB {
	config.NewConfig("config_test")
	pg, err := NewPGDB()
	if err != nil {
		t.Errorf("Could no open DataStore got error: %s", err)
	}
	err = pg.DeleteTables()
	if err != nil {
		t.Errorf("Could not Create Data Store got error: %s", err)
	}
	err = pg.CreateTables()
	if err != nil {
		t.Errorf("Could not Create Data Store got error: %s", err)
	}
	return pg
}

func deleteAndCloseTestDb(pg *PGDB, t *testing.T) {
	err := pg.DeleteTables()
	if err != nil {
		t.Errorf("could not delete datastore: %s", err)
	}
	if err = pg.Close(); err != nil {
		t.Errorf("could not close datastore: %s", err)
	}
}

func TestDB(t *testing.T) {
	//This test lacks a bunch of test where we expect errors
	ds := openAndCreateTestDb(t)
	deleteAndCloseTestDb(ds, t)
}
