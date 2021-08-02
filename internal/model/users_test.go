package model

import (
	"encoding/json"
	"fmt"
	"testing"
)

type TestConfig struct {
	StringConf string
	IntConf    int
	BoolConf   bool
}

func toJson(t TestConfig) string {
	b, _ := json.Marshal(t)
	return string(b)
}

func fromJson(str string) (TestConfig, error) {
	var t TestConfig
	err := json.Unmarshal([]byte(str), &t)
	return t, err
}

func checkEmptyJson(str string) error {
	var obj map[string]interface{}
	err := json.Unmarshal([]byte(str), &obj)
	if err != nil {
		return err
	} else if len(obj) != 0 {
		return fmt.Errorf("object is not empty: %v", len(obj))
	} else {
		return nil
	}
}

func TestUsers(t *testing.T) {
	ds, err := openTestDb()
	if err != nil {
		t.Errorf("could not create db: %s", err.Error())
	}
	err = ds.CreateUserStore()
	if err != nil {
		t.Errorf("could not create UserStore: %s", err.Error())
	}
	user, err := ds.User()
	if err != nil {
		t.Errorf("no user created: %s", err.Error())
	}
	//verify empty user
	cmpUser := User{}
	if *user != cmpUser {
		t.Errorf("initial user is not empty: %v", user)
	}

	//Update User
	cmpUser.Name = "test"
	cmpUser.Pic = "somepic"
	cmpUser.Bio = "somebio"

	ret, err := ds.UpdateUser(&cmpUser)
	if err != nil {
		t.Errorf("Could not update user: %s", err.Error())
	}
	if *ret != cmpUser {
		t.Errorf("Users dont match")
	}

	//Check that driveId and driveFolderName cannot be updated
	cmpUser.DriveFolderId = "someid"
	cmpUser.DriveFolderName = "somefoldername"

	ret, err = ds.UpdateUser(&cmpUser)
	if err != nil {
		t.Errorf("Could not update user: %s", err.Error())
	}
	cmpUser.DriveFolderName = ""
	cmpUser.DriveFolderId = ""
	if *ret != cmpUser {
		t.Errorf("driveId or driveFolderName accidently set")
	}

	//Check User Config (should be a json string)

	//Inital config should be an empty json object
	conf, err := ds.UserConfig()
	if err != nil {
		t.Errorf("Could not retrive user config: %s", err.Error())
	} else if checkEmptyJson(conf) != nil {
		t.Error(err.Error())
	}
	//Updating config
	expConfig := TestConfig{"strConf", 1, true}
	err = ds.UpdateUserConfig(toJson(expConfig))
	if err != nil {
		t.Errorf("Could not update config: %s", err.Error())
	}
	conf, err = ds.UserConfig()
	if err != nil {
		t.Errorf("Could not retrieve updated config: %s", err.Error())
	}
	actConfig, err := fromJson(conf)
	if err != nil {
		t.Errorf("Could not parse json config: %s", err.Error())
	} else if actConfig != expConfig {
		t.Errorf("Actual config is different from expected config")
	}

	//Dont accept non json config
	if err = ds.UpdateUserConfig("some non json string"); err == nil {
		t.Error("UserConfig accepted some non json string")
	}

	err = ds.DeleteUserStore()
	if err != nil {
		t.Errorf("Could not delete UserStore: %s", err)

	}

}
