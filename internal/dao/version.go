package dao

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"strings"
)

type VersionPG struct {
	db                *sqlx.DB
	versionFields     []string
	updateVersionStmt string
	getVersionStmt    string
}

func NewVersionPG(db *sqlx.DB) *VersionPG {
	v := &Version{}
	fields := getStructFields(v)
	uStmt := buildUpdateNamed2("version", fields, "")
	gStmt := fmt.Sprintf("SELECT %s FROM version LIMIT 1", strings.Join(fields, ","))
	return &VersionPG{db, fields, uStmt, gStmt}
}

func (dao *VersionPG) Update() (*Version, error) {
	v := Version{DbVersion, DbDescription}
	if _, err := dao.db.NamedExec(dao.updateVersionStmt, &v); err != nil {
		return nil, err
	}
	return dao.Get()
}

func (dao *VersionPG) Get() (*Version, error) {
	v := Version{}
	err := dao.db.Get(&v, dao.getVersionStmt)
	if err != nil {
		return nil, err
	} else {
		return &v, err
	}
}

func (dao *VersionPG) IsCurrent() (bool, error) {
	if v, err := dao.Get(); err != nil {
		return false, err
	} else {
		return v.VersionId == DbVersion, nil
	}

}
