package model

import (
	"github.com/msvens/mphotos/internal/config"
	"testing"
)

func openAndCreateTestDb(t *testing.T) DataStore {
	config.NewConfig("config_test")
	ds, err := NewDB()
	if err != nil {
		t.Errorf("Could no open DataStore got error: %s", err)
	}
	err = ds.CreateDataStore()
	if err != nil {
		t.Errorf("Could not Create Data Store got error: %s", err)
	}
	return ds
}

func deleteAndCloseTestDb(ds DataStore, t *testing.T) {
	err := ds.DeleteDataStore()
	if err != nil {
		t.Errorf("could not delete datastore: %s", err)
	}
	if err = ds.CloseDb(); err != nil {
		t.Errorf("could not close datastore: %s", err)
	}
}

func TestDB(t *testing.T) {
	//This test lacks a bunch of test where we expect errors
	ds := openAndCreateTestDb(t)
	deleteAndCloseTestDb(ds, t)
}
