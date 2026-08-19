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

// Update stamps the version row to the binary's current DbVersion. Used when
// creating a fresh database.
func (dao *VersionPG) Update() (*Version, error) {
	return dao.Set(DbVersion)
}

// Set stamps the version row to an explicit version. UpgradeDb calls it after
// each migration step so the marker advances one version at a time and a crash
// mid-upgrade leaves a consistent, resumable version.
func (dao *VersionPG) Set(versionId int) (*Version, error) {
	v := Version{versionId, DbDescription}
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

// Check verifies the database schema matches the binary's DbVersion, returning a
// descriptive error (it never mutates) when the database is behind, ahead, or
// unreadable. The server calls this at startup to refuse a mismatched schema up
// front instead of failing at runtime on a missing column. It does not upgrade —
// migrations stay a deliberate, separate `db upgrade` step.
func (dao *VersionPG) Check() error {
	v, err := dao.Get()
	if err != nil {
		return fmt.Errorf("could not read database version (is the database initialized? run `db create` / `db upgrade`): %w", err)
	}
	switch {
	case v.VersionId == DbVersion:
		return nil
	case v.VersionId < DbVersion:
		return fmt.Errorf("database schema is at version %d but this binary requires %d; run `db upgrade` before starting the server", v.VersionId, DbVersion)
	default:
		return fmt.Errorf("database schema is at version %d, newer than this binary (%d); deploy the matching build", v.VersionId, DbVersion)
	}
}
