package dao

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGuestProfileAndReap(t *testing.T) {
	pgdb := openAndCreateTestDb(t)
	defer deleteAndCloseTestDb(pgdb, t)

	// Add starts a guest unverified.
	g, err := pgdb.Guest.Add("nick", "nick@example.com")
	if err != nil {
		t.Fatalf("could not add guest: %v", err)
	}
	if g.Verified {
		t.Error("new guest should start unverified")
	}
	if !pgdb.Guest.HasByEmail("nick@example.com") || !pgdb.Guest.HasByName("nick") {
		t.Error("guest lookups by email/name should succeed")
	}

	// HasVerified is false until verified.
	if pgdb.Guest.HasVerified(g.Id) {
		t.Error("HasVerified should be false for an unverified guest")
	}
	if _, err := pgdb.Guest.Verify(g.Id); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if !pgdb.Guest.HasVerified(g.Id) {
		t.Error("HasVerified should be true after Verify")
	}
	if pgdb.Guest.HasVerified(uuid.New()) {
		t.Error("HasVerified should be false for a nonexistent guest")
	}

	// UpdateProfile sets the mutable fields; email stays put.
	upd, err := pgdb.Guest.UpdateProfile(g.Id, "nick2", "Nick Real", "hi there")
	if err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	if upd.Name != "nick2" || upd.FullName != "Nick Real" || upd.Description != "hi there" {
		t.Errorf("profile not updated: %+v", upd)
	}
	if upd.Email != "nick@example.com" {
		t.Errorf("email should be unchanged, got %q", upd.Email)
	}

	// List returns the guest.
	if list, err := pgdb.Guest.List(); err != nil {
		t.Fatalf("List failed: %v", err)
	} else if len(list) != 1 || list[0].Id != g.Id {
		t.Errorf("expected [%v], got %+v", g.Id, list)
	}

	// Reap: a stale unverified guest is removed; verified/fresh ones survive.
	stale, _ := pgdb.Guest.Add("stale", "stale@example.com")
	if _, err := pgdb.db.Exec("UPDATE guest SET verifytime = $1 WHERE id = $2",
		time.Now().Add(-48*time.Hour), stale.Id); err != nil {
		t.Fatalf("could not age stale guest: %v", err)
	}
	n, err := pgdb.Guest.DeleteUnverifiedBefore(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("reap failed: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 reaped, got %d", n)
	}
	if pgdb.Guest.Has(stale.Id) {
		t.Error("stale unverified guest should be gone")
	}
	if !pgdb.Guest.Has(g.Id) {
		t.Error("verified guest should survive the reap")
	}
}

func TestGuestDeleteCascade(t *testing.T) {
	pgdb := openAndCreateTestDb(t)
	defer deleteAndCloseTestDb(pgdb, t)

	g, _ := pgdb.Guest.Add("casc", "casc@example.com")
	photoId := uuid.New()
	if err := pgdb.Reaction.Add(&Reaction{GuestId: g.Id, PhotoId: photoId, Kind: "like"}); err != nil {
		t.Fatalf("could not add reaction: %v", err)
	}
	if err := pgdb.GuestCode.Issue(g.Id, GuestCodeSignup, "tok", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("could not issue code: %v", err)
	}

	if err := pgdb.Guest.Delete(g.Id); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if pgdb.Guest.Has(g.Id) {
		t.Error("guest should be deleted")
	}
	if pgdb.Reaction.Has(g.Id, photoId) {
		t.Error("reaction should be cascaded away")
	}
	if _, ok, _ := pgdb.GuestCode.FindSignup("tok"); ok {
		t.Error("guest code should be cascaded away")
	}
}

func TestGuestCodes(t *testing.T) {
	pgdb := openAndCreateTestDb(t)
	defer deleteAndCloseTestDb(pgdb, t)

	g, _ := pgdb.Guest.Add("codeguest", "code@example.com")

	// Signup token: found while valid, resolves to the guest.
	if err := pgdb.GuestCode.Issue(g.Id, GuestCodeSignup, "signup-token", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("issue signup failed: %v", err)
	}
	if id, ok, _ := pgdb.GuestCode.FindSignup("signup-token"); !ok || id != g.Id {
		t.Errorf("FindSignup should resolve to guest, got ok=%v id=%v", ok, id)
	}
	if _, ok, _ := pgdb.GuestCode.FindSignup("wrong-token"); ok {
		t.Error("FindSignup should miss on a wrong token")
	}

	// Login code: single-use consume.
	if err := pgdb.GuestCode.Issue(g.Id, GuestCodeLogin, "654321", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("issue login failed: %v", err)
	}
	if ok, _ := pgdb.GuestCode.ConsumeLogin(g.Id, "000000"); ok {
		t.Error("consume should fail on wrong code")
	}
	if ok, _ := pgdb.GuestCode.ConsumeLogin(g.Id, "654321"); !ok {
		t.Error("consume should succeed on correct code")
	}
	if ok, _ := pgdb.GuestCode.ConsumeLogin(g.Id, "654321"); ok {
		t.Error("consume should fail the second time (single use)")
	}

	// Issue overwrites the previous code for (guest, purpose).
	_ = pgdb.GuestCode.Issue(g.Id, GuestCodeSignup, "first", time.Now().Add(time.Hour))
	_ = pgdb.GuestCode.Issue(g.Id, GuestCodeSignup, "second", time.Now().Add(time.Hour))
	if _, ok, _ := pgdb.GuestCode.FindSignup("first"); ok {
		t.Error("old signup token should be overwritten")
	}
	if _, ok, _ := pgdb.GuestCode.FindSignup("second"); !ok {
		t.Error("latest signup token should be active")
	}

	// Expiry: an expired code is neither found nor consumable, and is purged.
	_ = pgdb.GuestCode.Issue(g.Id, GuestCodeLogin, "999999", time.Now().Add(-time.Minute))
	if ok, _ := pgdb.GuestCode.ConsumeLogin(g.Id, "999999"); ok {
		t.Error("expired login code should not consume")
	}
	_ = pgdb.GuestCode.Issue(g.Id, GuestCodeSignup, "expired-tok", time.Now().Add(-time.Minute))
	if _, ok, _ := pgdb.GuestCode.FindSignup("expired-tok"); ok {
		t.Error("expired signup token should not be found")
	}
	if n, err := pgdb.GuestCode.DeleteExpiredBefore(time.Now()); err != nil {
		t.Fatalf("DeleteExpiredBefore failed: %v", err)
	} else if n < 1 {
		t.Errorf("expected to purge at least 1 expired code, got %d", n)
	}
}
