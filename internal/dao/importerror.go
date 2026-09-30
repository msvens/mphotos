package dao

import (
	"time"

	"github.com/jmoiron/sqlx"
)

type ImportErrorPG struct {
	db *sqlx.DB
}

func NewImportErrorPG(db *sqlx.DB) *ImportErrorPG {
	return &ImportErrorPG{db}
}

// Record upserts a per-file import failure keyed by the source md5. Re-failing the
// same bytes updates the existing row rather than erroring, so the newest reason
// and timestamp win.
func (dao *ImportErrorPG) Record(e *ImportError) error {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	_, err := dao.db.Exec(
		`INSERT INTO import_error (md5, driveId, name, category, message, time)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (md5) DO UPDATE SET
		   driveId = EXCLUDED.driveId, name = EXCLUDED.name,
		   category = EXCLUDED.category, message = EXCLUDED.message, time = EXCLUDED.time`,
		e.Md5, e.DriveId, e.Name, e.Category, e.Message, e.Time)
	return err
}

// HasMd5 reports whether a source with this md5 has already failed to import, so
// callers can skip it instead of re-downloading/re-processing on every sync.
func (dao *ImportErrorPG) HasMd5(md5 string) bool {
	if rows, err := dao.db.Query("SELECT 1 FROM import_error WHERE md5 = $1", md5); err == nil {
		defer func() { _ = rows.Close() }()
		return rows.Next()
	}
	return false
}

// List returns all recorded import failures, most recent first.
func (dao *ImportErrorPG) List() ([]*ImportError, error) {
	ret := []*ImportError{}
	err := dao.db.Select(&ret, "SELECT * FROM import_error ORDER BY time DESC")
	return ret, err
}

// Delete removes a recorded failure by md5, e.g. to allow a retry.
func (dao *ImportErrorPG) Delete(md5 string) error {
	_, err := dao.db.Exec("DELETE FROM import_error WHERE md5 = $1", md5)
	return err
}
