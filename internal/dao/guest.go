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
	if res, err := dao.db.Exec("DELETE FROM guests WHERE id = $1", id); err != nil {
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
		defer rows.Close()
		return rows.Next()
	} else {
		return false
	}
}
func (dao *GuestPG) HasByEmail(email string) bool {
	if rows, err := dao.db.Query("SELECT 1 FROM guest WHERE email = $1", email); err == nil {
		defer rows.Close()
		return rows.Next()
	} else {
		return false
	}
}
func (dao *GuestPG) HasByName(name string) bool {
	if rows, err := dao.db.Query("SELECT 1 FROM guest WHERE name = $1", name); err == nil {
		defer rows.Close()
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
