package dao

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
	pgdb := openAndCreateTestDb(t)

	user, err := pgdb.User.Get()

	if err != nil {
		t.Errorf("no user created: %s", err.Error())
	}
	//verify empty user
	cmpUser := User{}
	cmpUser.Config = "{}"
	if *user != cmpUser {
		t.Errorf("initial user is not empty: %v %v", user, cmpUser)
	}

	//Update User
	cmpUser.Name = "test"
	cmpUser.Pic = "somepic"
	cmpUser.Bio = "somebio"

	ret, err := pgdb.User.Update(&cmpUser)
	if err != nil {
		t.Errorf("Could not update user: %s", err.Error())
	}
	if *ret != cmpUser {
		t.Errorf("Users dont match")
	}

	//Inital config should be an empty json object
	u, err := pgdb.User.Get()
	if err != nil {
		t.Errorf("Could not retrive user config: %s", err.Error())
	} else if e := checkEmptyJson(u.Config); e != nil {
		t.Error(e.Error())
	}
	//Updating config
	expConfig := TestConfig{"strConf", 1, true}
	u.Config = toJson(expConfig)
	_, err = pgdb.User.Update(u)
	if err != nil {
		t.Errorf("Could not update config: %s", err.Error())
	}
	u, err = pgdb.User.Get()
	if err != nil {
		t.Errorf("Could not retrieve updated config: %s", err.Error())
	}
	actConfig, err := fromJson(u.Config)
	if err != nil {
		t.Errorf("Could not parse json config: %s", err.Error())
	} else if actConfig != expConfig {
		t.Errorf("Actual config is different from expected config")
	}

	//Dont accept non json config
	u.Config = "some non json string"
	if _, err = pgdb.User.Update(u); err == nil {
		t.Error("UserConfig accepted some non json string")
	}

	deleteAndCloseTestDb(pgdb, t)
}
