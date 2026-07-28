package dao

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestUpgradeToV5 rewinds a freshly created (v5) database to the v4 shape — no
// fullname/description columns, no guestcode table, version 4, with an
// unverified legacy guest — then runs upgradeToV5 and asserts the schema is
// extended and every existing guest is grandfathered to verified=true. The
// drop/recreate harness only ever creates the latest schema, so this is the only
// coverage of the migration path.
func TestUpgradeToV5(t *testing.T) {
	pgdb := openAndCreateTestDb(t)
	defer deleteAndCloseTestDb(pgdb, t)

	// Rewind to the v4 guest shape and insert a legacy, unverified guest.
	for _, stmt := range []string{
		"ALTER TABLE guest DROP COLUMN fullname",
		"ALTER TABLE guest DROP COLUMN description",
		"DROP TABLE IF EXISTS guestcode",
		"UPDATE version SET versionId = 4",
	} {
		if _, err := pgdb.db.Exec(stmt); err != nil {
			t.Fatalf("rewind step %q failed: %v", stmt, err)
		}
	}
	gid := uuid.New()
	if _, err := pgdb.db.Exec(
		"INSERT INTO guest (id, name, email, verified, verifytime) VALUES ($1,$2,$3,$4,$5)",
		gid, "legacy", "legacy@example.com", false, time.Now()); err != nil {
		t.Fatalf("could not insert legacy guest: %v", err)
	}

	if err := upgradeToV5(pgdb); err != nil {
		t.Fatalf("upgradeToV5 failed: %v", err)
	}

	// Version bumped.
	if v, err := pgdb.Version.Get(); err != nil {
		t.Fatalf("could not read version: %v", err)
	} else if v.VersionId != DbVersion {
		t.Errorf("expected version %d got %d", DbVersion, v.VersionId)
	}

	// Legacy guest preserved and grandfathered to verified, with empty new fields.
	g, err := pgdb.Guest.Get(gid)
	if err != nil {
		t.Fatalf("legacy guest not preserved: %v", err)
	}
	if !g.Verified {
		t.Error("expected legacy guest to be grandfathered to verified=true")
	}
	if g.FullName != "" || g.Description != "" {
		t.Errorf("expected empty new profile fields, got fullName=%q description=%q", g.FullName, g.Description)
	}

	// guestcode table exists and is usable after the upgrade.
	if err := pgdb.GuestCode.Issue(gid, GuestCodeLogin, "123456", time.Now().Add(time.Minute)); err != nil {
		t.Errorf("guestcode table not usable after upgrade: %v", err)
	}
}
