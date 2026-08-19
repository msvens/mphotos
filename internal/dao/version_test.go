package dao

import (
	"strings"
	"testing"
)

// TestVersionCheck covers the startup guard: a current database passes, a behind
// database fails with a message pointing at `db upgrade`, and a database newer
// than the binary also fails. Guards against the silent runtime failure a
// schema-behind server produced (a missing column only blew up mid-request).
func TestVersionCheck(t *testing.T) {
	pgdb := openAndCreateTestDb(t)
	defer deleteAndCloseTestDb(pgdb, t)

	// A freshly created db is stamped at DbVersion, so Check passes.
	if err := pgdb.Version.Check(); err != nil {
		t.Fatalf("Check should pass on a current db, got: %v", err)
	}

	// Behind the binary -> error that tells the operator to upgrade.
	if _, err := pgdb.Version.Set(DbVersion - 1); err != nil {
		t.Fatalf("could not set version behind: %v", err)
	}
	err := pgdb.Version.Check()
	if err == nil {
		t.Fatal("Check should fail when the db is behind the binary")
	}
	if !strings.Contains(err.Error(), "db upgrade") {
		t.Errorf("behind error should point to `db upgrade`, got: %v", err)
	}

	// Newer than the binary -> error (an old binary must not run a newer schema).
	if _, err := pgdb.Version.Set(DbVersion + 1); err != nil {
		t.Fatalf("could not set version ahead: %v", err)
	}
	if err := pgdb.Version.Check(); err == nil {
		t.Error("Check should fail when the db is newer than the binary")
	}

	// Restore the marker so teardown leaves a consistent db.
	if _, err := pgdb.Version.Set(DbVersion); err != nil {
		t.Fatalf("could not restore version: %v", err)
	}
}
