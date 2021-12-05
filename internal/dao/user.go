package dao

import (
	"encoding/json"
	"fmt"
	"github.com/jmoiron/sqlx"
	"strings"
)

type UserPG struct {
	db             *sqlx.DB
	userFields     []string
	updateUserStmt string
	getUserStmt    string
}

func NewUserPG(db *sqlx.DB) *UserPG {
	u := &User{}
	fields := getStructFields(u)
	uStmt := buildUpdateNamed2("usert", fields, "")
	gStmt := fmt.Sprintf("SELECT %s FROM usert LIMIT 1", strings.Join(fields, ","))
	return &UserPG{db, fields, uStmt, gStmt}
}

func (dao *UserPG) Update(u *User) (*User, error) {
	if !json.Valid([]byte(u.Config)) {
		return nil, fmt.Errorf("Non valid json config: %s", u.Config)
	}
	if _, err := dao.db.NamedExec(dao.updateUserStmt, u); err != nil {
		return nil, err
	}
	//return u, nil
	return dao.Get()
}

func (dao *UserPG) Get() (*User, error) {
	u := User{}
	err := dao.db.Get(&u, dao.getUserStmt)
	if err != nil {
		return nil, err
	} else {
		return &u, err
	}
}
