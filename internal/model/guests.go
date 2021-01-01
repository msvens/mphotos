package model

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

type Guest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Verify struct {
	Verified bool      `json:"verified"`
	Time     time.Time `json:"time"`
}

type GuestStore interface {
	CreateGuestStore() error
	CreateLikeStore() error
	DeleteGuestStore() error
	DeleteLikeStore() error
	AddGuest(id uuid.UUID, u *Guest) error
	VerifyGuest(id uuid.UUID) (*Verify, error)
	Guest(id uuid.UUID) (*Guest, error)
	GuestByEmail(email string) (*Guest, error)
	GuestUUID(email string) (*uuid.UUID, error)
	HasGuest(id uuid.UUID) bool
	HasGuestByEmail(email string) bool
	Verified(id uuid.UUID) (*Verify, error)
	AddLike(guest uuid.UUID, driveId string) error
	DeleteLike(guest uuid.UUID, driveId string) error
	Like(guest uuid.UUID, driveId string) bool
	PhotoLikes(driveId string) ([]*Guest, error)
	GuestLikes(guest uuid.UUID) ([]string, error)

	//GuestLikes(uuid uuid.UUID) ([]*string, error)
}

func (db *DB) CreateGuestStore() error {
	const stmt = `
CREATE TABLE IF NOT EXISTS guests (
	id UUID PRIMARY KEY,
	name TEXT NOT NULL,
	email TEXT NOT NULL,
	verified BOOLEAN NOT NULL,
	verifytime TIMESTAMP NOT NULL,
	CONSTRAINT guestemail UNIQUE (email)
);
`
	_, err := db.Exec(stmt)
	fmt.Println("create guest store, ", err)
	return err
}

func (db *DB) CreateLikeStore() error {
	const stmt = `
	CREATE TABLE IF NOT EXISTS likes (
		guest UUID,
		driveId TEXT,
		PRIMARY KEY (guest, driveId)
	);
`
	_, err := db.Exec(stmt)
	return err
}

func (db *DB) DeleteGuestStore() error {
	_, err := db.Exec("DROP TABLE IF EXISTS guests;")
	return err
}

func (db *DB) DeleteLikeStore() error {
	_, err := db.Exec("DROP TABLE IF EXISTS likes;")
	return err
}

func (db *DB) AddGuest(uuid uuid.UUID, g *Guest) error {
	now := time.Now()
	const stmt = "INSERT INTO guests (id, name, email, verified, verifytime) VALUES ($1, $2, $3, $4, $5)"
	if _, err := db.Exec(stmt, uuid.String(), g.Name, g.Email, false, now); err != nil {
		fmt.Println("create guest failed ", err)
		return err
	} else {
		fmt.Println("create guest succeded ", err)
		return nil
	}
}

func (db *DB) DeleteGuest(uuid uuid.UUID) (bool, error) {
	const delGuest = "DELETE FROM guests WHERE id = $1;"
	var cnt int64
	if res, err := db.Exec(delGuest, uuid.String()); err != nil {
		return false, err
	} else {
		cnt, _ = res.RowsAffected()
	}

	const delLikes = "DELETE FROM likes WHERE guest = $1"

	if _, err := db.Exec(delLikes, uuid.String()); err != nil {
		return false, err
	}

	return cnt > 0, nil

}

func (db *DB) Guest(uuid uuid.UUID) (*Guest, error) {
	const stmt = "SELECT name, email FROM guests WHERE id = $1"
	resp := Guest{}
	r := db.QueryRow(stmt, uuid.String())
	if err := r.Scan(&resp.Name, &resp.Email); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (db *DB) GuestByEmail(email string) (*Guest, error) {
	const stmt = "SELECT name, email FROM guests WHERE email = $1"
	resp := Guest{}
	r := db.QueryRow(stmt, email)
	if err := r.Scan(&resp.Name, &resp.Email); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (db *DB) GuestUUID(email string) (*uuid.UUID, error) {
	const stmt = "SELECT id FROM guests WHERE email = $1"
	var uuidStr string
	r := db.QueryRow(stmt, email)
	if err := r.Scan(&uuidStr); err != nil {
		return nil, err
	}
	if uuid, err := uuid.Parse(uuidStr); err != nil {
		return nil, err
	} else {
		return &uuid, nil
	}
}

func (db *DB) HasGuest(guest uuid.UUID) bool {
	const stmt = "SELECT 1 FROM guests WHERE id = $1"
	if rows, err := db.Query(stmt, guest.String()); err == nil {
		defer rows.Close()
		return rows.Next()
	} else {
		return false
	}
}

func (db *DB) HasGuestByEmail(email string) bool {
	const stmt = "SELECT 1 FROM guests WHERE email = $1"
	if rows, err := db.Query(stmt, email); err == nil {
		defer rows.Close()
		return rows.Next()
	} else {
		return false
	}
}

func (db *DB) Verified(uuid uuid.UUID) (*Verify, error) {
	const stmt = "SELECT verified, verifytime FROM guests WHERE id = $1"
	var ver Verify
	r := db.QueryRow(stmt, uuid.String())
	err := r.Scan(&ver.Verified, &ver.Time)
	fmt.Println(ver)
	return &ver, err
}

func (db *DB) VerifyGuest(uuid uuid.UUID) (*Verify, error) {
	const stmt = "UPDATE guests SET (verified, verifytime) = ($1, $2) WHERE id = $3"
	if _, err := db.Exec(stmt, true, time.Now(), uuid.String()); err != nil {
		return nil, err
	}
	return db.Verified(uuid)
}

func (db *DB) AddLike(uuid uuid.UUID, driveId string) error {
	const stmt = "INSERT INTO likes (guest, driveId) VALUES ($1, $2) ON CONFLICT DO NOTHING"
	if _, err := db.Exec(stmt, uuid.String(), driveId); err != nil {
		return err
	} else {
		return nil
	}
}

func (db *DB) DeleteLike(uuid uuid.UUID, driveId string) error {
	const stmt = "DELETE FROM likes WHERE guest = $1 AND driveId = $2"
	if _, err := db.Exec(stmt, uuid.String(), driveId); err != nil {
		return err
	} else {
		return nil
	}
}

func (db *DB) Like(guest uuid.UUID, driveId string) bool {
	const stmt = "SELECT 1 FROM likes WHERE guest = $1 AND driveId = $2"
	if rows, err := db.Query(stmt, guest.String(), driveId); err == nil {
		defer rows.Close()
		return rows.Next()
	} else {
		return false
	}
}

func (db *DB) GuestLikes(guest uuid.UUID) ([]string, error) {
	const stmt = "SELECT driveId FROM likes WHERE guest = $1"
	photos := []string{}
	if rows, err := db.Query(stmt, guest.String()); err != nil {
		return nil, err
	} else {
		defer rows.Close()
		for rows.Next() {
			var driveId string
			if err := rows.Scan(&driveId); err != nil {
				return nil, err
			}
			photos = append(photos, driveId)
		}
	}
	return photos, nil
}

func (db *DB) PhotoLikes(driveId string) ([]*Guest, error) {
	const stmt = "SELECT name,email FROM guests WHERE id IN (SELECT guest FROM likes WHERE driveId = $1)"
	guests := []*Guest{}
	if rows, err := db.Query(stmt, driveId); err != nil {
		return nil, err
	} else {
		defer rows.Close()
		for rows.Next() {
			var guest = Guest{}
			if err := rows.Scan(&guest.Name, &guest.Email); err != nil {
				return nil, err
			}
			guests = append(guests, &guest)
		}
	}
	fmt.Println(len(guests))
	return guests, nil
}
