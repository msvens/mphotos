package dao

import (
	"encoding/json"
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
			return upgradeToV4(pgdb)
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

func upgradeToV4(pgdb *PGDB) error {
	logger.Infow("Upgrading Db to Version", "version", DbVersion)

	if _, err := pgdb.db.Exec(schemaV3toV4); err != nil {
		return err
	}

	logger.Info("Db Updated. Change Version Info")
	if v, err := pgdb.Version.Update(); err != nil {
		return err
	} else {
		logger.Infow("Updated Db to version", "version", v.VersionId)
	}

	// Seed the new typed column once from the client config blob — the only place
	// the photostream album id lived before this version. This is the single
	// appropriate spot to read that blob; the running server never does.
	var config string
	if err := pgdb.db.Get(&config, "SELECT config FROM usert LIMIT 1"); err != nil {
		logger.Warnw("could not read user config while seeding photostream id; leaving it unset", "error", err)
		return nil
	}
	var conf map[string]interface{}
	if err := json.Unmarshal([]byte(config), &conf); err != nil {
		logger.Warnw("could not parse user config while seeding photostream id; leaving it unset", "error", err)
		return nil
	}
	if id, ok := conf["photoStreamAlbumId"].(string); ok && id != "" {
		if _, err := pgdb.db.Exec("UPDATE usert SET photostreamalbumid = $1", id); err != nil {
			return err
		}
		logger.Infow("seeded photostream album id from config", "id", id)
	}
	return nil
}
