package dao

import (
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Guest code purposes.
const (
	GuestCodeSignup = "signup"
	GuestCodeLogin  = "login"
)

// GuestCodePG stores short-lived one-time codes for guest signup verification
// (a link token) and login (a typed numeric code). At most one active code
// exists per (guest, purpose); issuing again overwrites the previous one.
type GuestCodePG struct {
	db *sqlx.DB
}

func NewGuestCodePG(db *sqlx.DB) *GuestCodePG {
	return &GuestCodePG{db}
}

// Issue stores (or replaces) the code for a (guest, purpose) with an expiry.
func (dao *GuestCodePG) Issue(guestId uuid.UUID, purpose, code string, expires time.Time) error {
	const stmt = `INSERT INTO guestcode (guestid, purpose, code, expires) VALUES ($1, $2, $3, $4)
		ON CONFLICT (guestid, purpose) DO UPDATE SET code = EXCLUDED.code, expires = EXCLUDED.expires`
	_, err := dao.db.Exec(stmt, guestId, purpose, code, expires)
	return err
}

// FindSignup resolves a signup token to its guest, provided the token is
// unexpired. The token is unguessable (32 random bytes), so a lookup by code
// alone is safe. Returns (id, true, nil) on a valid match.
func (dao *GuestCodePG) FindSignup(code string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := dao.db.Get(&id,
		"SELECT guestid FROM guestcode WHERE purpose = $1 AND code = $2 AND expires > $3",
		GuestCodeSignup, code, time.Now())
	if err != nil {
		return uuid.Nil, false, nil
	}
	return id, true, nil
}

// ConsumeLogin verifies a login code for a guest and, on success, deletes it
// (single use). Returns true only when the code matched and was unexpired.
func (dao *GuestCodePG) ConsumeLogin(guestId uuid.UUID, code string) (bool, error) {
	res, err := dao.db.Exec(
		"DELETE FROM guestcode WHERE guestid = $1 AND purpose = $2 AND code = $3 AND expires > $4",
		guestId, GuestCodeLogin, code, time.Now())
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Delete removes the code for a (guest, purpose), e.g. a signup token once the
// guest is verified.
func (dao *GuestCodePG) Delete(guestId uuid.UUID, purpose string) error {
	_, err := dao.db.Exec("DELETE FROM guestcode WHERE guestid = $1 AND purpose = $2", guestId, purpose)
	return err
}

// DeleteExpiredBefore purges codes whose expiry is older than t.
func (dao *GuestCodePG) DeleteExpiredBefore(t time.Time) (int64, error) {
	res, err := dao.db.Exec("DELETE FROM guestcode WHERE expires < $1", t)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
