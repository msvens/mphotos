package dao

import (
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"time"
)

type GuestPG struct {
	db              *sqlx.DB
	guestFields     []string
	insertGuestStmt string
}

func NewGuestPG(db *sqlx.DB) *GuestPG {
	c := &Guest{}
	fields := getStructFields(c)
	stmt := buildInsertNamed("guest", fields)
	return &GuestPG{db, fields, stmt}
}

func (dao *GuestPG) Add(name, email string) (*Guest, error) {
	g := Guest{Id: uuid.New(), Name: name, Email: email, VerifyTime: time.Now(), Verified: false}
	if _, err := dao.db.NamedExec(dao.insertGuestStmt, g); err != nil {
		return nil, err
	}
	return dao.Get(g.Id)
}
func (dao *GuestPG) Delete(id uuid.UUID) error {
	var cnt int64
	if res, err := dao.db.Exec("DELETE FROM guest WHERE id = $1", id); err != nil {
		return err
	} else {
		cnt, _ = res.RowsAffected()
	}
	if cnt > 0 {
		if _, err := dao.db.Exec("DELETE from reaction WHERE guestId = $1", id); err != nil {
			return err
		}
		if _, err := dao.db.Exec("DELETE from comment WHERE guestId = $1", id); err != nil {
			return err
		}
		if _, err := dao.db.Exec("DELETE from guestcode WHERE guestid = $1", id); err != nil {
			return err
		}
	}
	return nil
}

func (dao *GuestPG) Verify(id uuid.UUID) (*Guest, error) {
	const stmt = "UPDATE guest SET (verified, verifytime) = ($1, $2) WHERE id = $3"
	if _, err := dao.db.Exec(stmt, true, time.Now(), id); err != nil {
		return nil, err
	}
	return dao.Get(id)
}
func (dao *GuestPG) Get(id uuid.UUID) (*Guest, error) {
	ret := Guest{}
	if err := dao.db.Get(&ret, "SELECT * FROM guest WHERE id = $1", id); err != nil {
		return nil, err
	}
	return &ret, nil
}

func (dao *GuestPG) GetByEmail(email string) (*Guest, error) {
	ret := Guest{}
	err := dao.db.Get(&ret, "SELECT * FROM guest WHERE email = $1", email)
	return &ret, err
}
func (dao *GuestPG) Has(id uuid.UUID) bool {
	if rows, err := dao.db.Query("SELECT 1 FROM guest WHERE id = $1", id); err == nil {
		defer func() { _ = rows.Close() }()
		return rows.Next()
	} else {
		return false
	}
}
func (dao *GuestPG) HasByEmail(email string) bool {
	if rows, err := dao.db.Query("SELECT 1 FROM guest WHERE email = $1", email); err == nil {
		defer func() { _ = rows.Close() }()
		return rows.Next()
	} else {
		return false
	}
}
func (dao *GuestPG) HasByName(name string) bool {
	if rows, err := dao.db.Query("SELECT 1 FROM guest WHERE name = $1", name); err == nil {
		defer func() { _ = rows.Close() }()
		return rows.Next()
	} else {
		return false
	}
}
func (dao *GuestPG) Update(email string, name string, id uuid.UUID) (*Guest, error) {
	const stmt = "UPDATE guest SET (email, name) = ($1, $2) WHERE id = $3"
	if _, err := dao.db.Exec(stmt, email, name, id); err != nil {
		return nil, err
	}
	return dao.Get(id)
}

// UpdateProfile updates the mutable profile fields. Email is the login identity
// and is intentionally not updatable here.
func (dao *GuestPG) UpdateProfile(id uuid.UUID, name, fullName, description string) (*Guest, error) {
	const stmt = "UPDATE guest SET (name, fullname, description) = ($1, $2, $3) WHERE id = $4"
	if _, err := dao.db.Exec(stmt, name, fullName, description, id); err != nil {
		return nil, err
	}
	return dao.Get(id)
}

// UpdateAvatar sets the guest's avatar reference (the file extension, e.g.
// ".jpg", or "" to clear it).
func (dao *GuestPG) UpdateAvatar(avatar string, id uuid.UUID) (*Guest, error) {
	if _, err := dao.db.Exec("UPDATE guest SET avatar = $1 WHERE id = $2", avatar, id); err != nil {
		return nil, err
	}
	return dao.Get(id)
}

// List returns all guests, for the owner-facing guest view.
func (dao *GuestPG) List() ([]*Guest, error) {
	ret := []*Guest{}
	err := dao.db.Select(&ret, "SELECT * FROM guest ORDER BY name")
	return ret, err
}

// HasVerified reports whether a guest exists AND is verified — the read-path
// check that gates guest-only routes.
func (dao *GuestPG) HasVerified(id uuid.UUID) bool {
	var verified bool
	if err := dao.db.Get(&verified, "SELECT verified FROM guest WHERE id = $1", id); err != nil {
		return false
	}
	return verified
}

// DeleteUnverifiedBefore removes guests that never verified and whose signup
// window (verifytime) is older than t, cascading to their comments/likes/codes.
// It returns the number of guests removed.
func (dao *GuestPG) DeleteUnverifiedBefore(t time.Time) (int64, error) {
	var ids []uuid.UUID
	if err := dao.db.Select(&ids, "SELECT id FROM guest WHERE verified = false AND verifytime < $1", t); err != nil {
		return 0, err
	}
	var n int64
	for _, id := range ids {
		if err := dao.Delete(id); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
