package dao

import (
	"fmt"
)

// for now this is just hard coded
func canUpgradeDb(pgdb *PGDB) bool {
	if v, err := pgdb.Version.Get(); err != nil {
		logger.Errorw("could not get Version info", "error", err)
		return false
	} else {
		return v.VersionId+1 == DbVersion
	}
}

func UpgradeDb() error {
	var err error
	var pgdb *PGDB
	if pgdb, err = NewPGDB(); err != nil {
		return err
	}
	hasVersion := pgdb.tableExists("version")
	hasPhotos := pgdb.tableExists("photos")
	if hasVersion {
		if isCurrent, err := pgdb.Version.IsCurrent(); err != nil {
			return err
		} else if isCurrent {
			logger.Info("Database is up to date")
			return nil
		} else if canUpgradeDb(pgdb) {
			return upgradeToV5(pgdb)
		} else {
			return fmt.Errorf("cannot upgrade database, wrong current version")
		}
	} else if hasPhotos {
		return fmt.Errorf("cannot upgrade database")
	} else {
		logger.Info("No database exists. Creating a fresh db")
		if err = pgdb.CreateTables(); err != nil {
			return err
		}
	}
	return nil
}

func upgradeToV5(pgdb *PGDB) error {
	logger.Infow("Upgrading Db to Version", "version", DbVersion)

	if _, err := pgdb.db.Exec(schemaV4toV5); err != nil {
		return err
	}

	// Grandfather every existing guest to verified=true. Under the old flow the
	// verified flag was cosmetic and never enforced, so legacy guests were fully
	// active regardless. Without this the new verified-only login would lock them
	// out and the reaper would delete them (and their comments/likes) on first run.
	res, err := pgdb.db.Exec("UPDATE guest SET verified = true WHERE verified = false")
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		logger.Infow("grandfathered existing guests to verified", "count", n)
	}

	logger.Info("Db Updated. Change Version Info")
	if v, err := pgdb.Version.Update(); err != nil {
		return err
	} else {
		logger.Infow("Updated Db to version", "version", v.VersionId)
	}
	return nil
}
