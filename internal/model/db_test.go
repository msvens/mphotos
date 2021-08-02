package model

import (
	"github.com/msvens/mphotos/internal/config"
	"testing"
)

func openTestDb() (DataStore, error) {
	config.NewConfig("config_test")
	return NewDB()
}

func TestDB(t *testing.T) {
	ds, err := openTestDb()
	if err != nil {
		t.Errorf("could not create db: %s", err.Error())
	}
	if err = ds.CreateDataStore(); err != nil {
		t.Errorf("could not create data store: %s", err.Error())
	}
	if err = ds.DeleteDataStore(); err != nil {
		t.Errorf("could not delete data store: %s", err.Error())

	}
	if err = ds.CloseDb(); err != nil {
		t.Errorf("could not close db: %s", err.Error())
	}
}
