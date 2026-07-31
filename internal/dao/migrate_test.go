package dao

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestUpgradeWalk rewinds a freshly created (v5) database to the v4 shape — no
// fullname/description columns, no guestcode table, version 4, with an
// unverified legacy guest — then runs the real UpgradeDb walk and asserts the
// schema is extended, the version is stamped, and every existing guest is
// grandfathered to verified=true. The drop/recreate harness only ever creates
// the latest schema, so this is the only coverage of the migration path.
func TestUpgradeWalk(t *testing.T) {
	pgdb := openAndCreateTestDb(t)
	defer deleteAndCloseTestDb(pgdb, t)

	// Rewind to the v4 guest shape and insert a legacy, unverified guest. This
	// exercises the multi-version walk: v4 -> v5 -> v6.
	for _, stmt := range []string{
		"ALTER TABLE guest DROP COLUMN fullname",
		"ALTER TABLE guest DROP COLUMN description",
		"ALTER TABLE guest DROP COLUMN avatar",
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

	// UpgradeDb walks v4 -> v5 via the migrations map and stamps the version.
	if err := UpgradeDb(); err != nil {
		t.Fatalf("UpgradeDb failed: %v", err)
	}

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
	if g.FullName != "" || g.Description != "" || g.Avatar != "" {
		t.Errorf("expected empty new profile fields, got fullName=%q description=%q avatar=%q", g.FullName, g.Description, g.Avatar)
	}

	// The v6 avatar column is present and writable.
	if _, err := pgdb.Guest.UpdateAvatar(".jpg", gid); err != nil {
		t.Errorf("avatar column not usable after upgrade: %v", err)
	}

	// guestcode table exists and is usable after the upgrade.
	if err := pgdb.GuestCode.Issue(gid, GuestCodeLogin, "123456", time.Now().Add(time.Minute)); err != nil {
		t.Errorf("guestcode table not usable after upgrade: %v", err)
	}
}

// TestUpgradeUpToDate: a database already at the latest version is a no-op.
func TestUpgradeUpToDate(t *testing.T) {
	pgdb := openAndCreateTestDb(t)
	defer deleteAndCloseTestDb(pgdb, t)

	if err := UpgradeDb(); err != nil {
		t.Fatalf("UpgradeDb on current db should be a no-op, got: %v", err)
	}
	if v, _ := pgdb.Version.Get(); v.VersionId != DbVersion {
		t.Errorf("expected version %d got %d", DbVersion, v.VersionId)
	}
}

// TestUpgradeNewerThanBinary: a database ahead of the binary must error, not
// silently proceed.
func TestUpgradeNewerThanBinary(t *testing.T) {
	pgdb := openAndCreateTestDb(t)
	defer deleteAndCloseTestDb(pgdb, t)

	if _, err := pgdb.Version.Set(DbVersion + 1); err != nil {
		t.Fatalf("could not set version ahead: %v", err)
	}
	if err := UpgradeDb(); err == nil {
		t.Error("expected an error upgrading a db newer than the binary")
	}
}
