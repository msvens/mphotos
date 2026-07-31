package dao

import (
	"fmt"
)

// migrations maps a target version to the function that upgrades the database
// from the immediately prior version to it (e.g. migrations[5] takes a v4 db to
// v5). Every step is kept here — never delete an entry — so the full upgrade
// chain is retained and any older database can be walked forward one version at
// a time. Add new steps here; do not replace old ones.
//
// A migration must only apply its schema/data changes; it must NOT stamp the
// version. UpgradeDb stamps each version after the step succeeds.
var migrations = map[int]func(*PGDB) error{
	5: upgradeToV5,
}

// UpgradeDb brings the database up to the binary's DbVersion, applying each
// intervening migration in order and stamping the version after each step (so a
// crash mid-walk leaves a consistent, resumable marker). A fresh database is
// created directly at the latest schema.
func UpgradeDb() error {
	var err error
	var pgdb *PGDB
	if pgdb, err = NewPGDB(); err != nil {
		return err
	}
	hasVersion := pgdb.tableExists("version")
	hasPhotos := pgdb.tableExists("photos")
	if !hasVersion {
		if hasPhotos {
			return fmt.Errorf("cannot upgrade database")
		}
		logger.Info("No database exists. Creating a fresh db")
		return pgdb.CreateTables()
	}

	v, err := pgdb.Version.Get()
	if err != nil {
		return err
	}
	if v.VersionId == DbVersion {
		logger.Info("Database is up to date")
		return nil
	}
	if v.VersionId > DbVersion {
		return fmt.Errorf("database version %d is newer than this binary (%d)", v.VersionId, DbVersion)
	}

	for target := v.VersionId + 1; target <= DbVersion; target++ {
		mig, ok := migrations[target]
		if !ok {
			return fmt.Errorf("no migration registered to reach version %d", target)
		}
		logger.Infow("Upgrading database", "from", target-1, "to", target)
		if err := mig(pgdb); err != nil {
			return err
		}
		if _, err := pgdb.Version.Set(target); err != nil {
			return err
		}
	}
	logger.Infow("Database upgraded", "version", DbVersion)
	return nil
}

// upgradeToV5 takes a v4 database to v5. It does not stamp the version — the
// UpgradeDb loop does that after the step succeeds.
func upgradeToV5(pgdb *PGDB) error {
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
	return nil
}
