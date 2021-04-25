package model

import (
	"github.com/msvens/mphotos/internal/config"
	"testing"
)

func openTestDb() {

}

func TestDB(t *testing.T) {
	config.NewConfig("config_test")
	ds, err := NewDB()
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
