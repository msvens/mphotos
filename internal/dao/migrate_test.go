package dao

import (
	"testing"

	"github.com/google/uuid"
)

// TestUpgradeToV4Seed rewinds a freshly created (v4) database to the v3 shape —
// no photostreamalbumid column, version 3, the album id living only in the config
// blob — then runs upgradeToV4 and asserts the column is seeded from that blob.
// The drop/recreate harness only ever creates the latest schema, so this is the
// only coverage of the migration seed path.
func TestUpgradeToV4Seed(t *testing.T) {
	pgdb := openAndCreateTestDb(t)
	defer deleteAndCloseTestDb(pgdb, t)

	streamID := uuid.New().String()

	// Rewind to v3: drop the new column, mark the db version 3, and stash the
	// photostream id in the config blob the way the old clients did.
	if _, err := pgdb.db.Exec("ALTER TABLE usert DROP COLUMN photostreamalbumid"); err != nil {
		t.Fatalf("could not drop column to simulate v3: %v", err)
	}
	if _, err := pgdb.db.Exec("UPDATE version SET versionId = 3"); err != nil {
		t.Fatalf("could not set version to 3: %v", err)
	}
	if _, err := pgdb.db.Exec(`UPDATE usert SET config = $1`,
		`{"photoStreamAlbumId":"`+streamID+`","photoGridCols":3}`); err != nil {
		t.Fatalf("could not seed config blob: %v", err)
	}

	if err := upgradeToV4(pgdb); err != nil {
		t.Fatalf("upgradeToV4 failed: %v", err)
	}

	// Version bumped.
	if v, err := pgdb.Version.Get(); err != nil {
		t.Fatalf("could not read version: %v", err)
	} else if v.VersionId != DbVersion {
		t.Errorf("expected version %d got %d", DbVersion, v.VersionId)
	}

	// Column exists and was seeded from the blob.
	user, err := pgdb.User.Get()
	if err != nil {
		t.Fatalf("could not read user after upgrade: %v", err)
	}
	if user.PhotoStreamAlbumId != streamID {
		t.Errorf("expected seeded photoStreamAlbumId %q got %q", streamID, user.PhotoStreamAlbumId)
	}
}

// TestUpgradeToV4NoSeed verifies the migration is resilient when the config blob
// has no photostream id: the column is added and left empty, not errored.
func TestUpgradeToV4NoSeed(t *testing.T) {
	pgdb := openAndCreateTestDb(t)
	defer deleteAndCloseTestDb(pgdb, t)

	if _, err := pgdb.db.Exec("ALTER TABLE usert DROP COLUMN photostreamalbumid"); err != nil {
		t.Fatalf("could not drop column to simulate v3: %v", err)
	}
	if _, err := pgdb.db.Exec("UPDATE version SET versionId = 3"); err != nil {
		t.Fatalf("could not set version to 3: %v", err)
	}
	if _, err := pgdb.db.Exec(`UPDATE usert SET config = '{}'`); err != nil {
		t.Fatalf("could not reset config blob: %v", err)
	}

	if err := upgradeToV4(pgdb); err != nil {
		t.Fatalf("upgradeToV4 failed: %v", err)
	}

	user, err := pgdb.User.Get()
	if err != nil {
		t.Fatalf("could not read user after upgrade: %v", err)
	}
	if user.PhotoStreamAlbumId != "" {
		t.Errorf("expected empty photoStreamAlbumId, got %q", user.PhotoStreamAlbumId)
	}
}
