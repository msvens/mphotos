package model

import "github.com/google/uuid"

type LikeStore interface {
	CreateLikeStore() error
	DeleteLikeStore() error
	AddLike(guest uuid.UUID, driveId string) error
	DeleteLike(guest uuid.UUID, driveId string) error
	DeleteLikes(guest uuid.UUID) error
	GuestLikes(guest uuid.UUID) ([]string, error)
	Like(guest uuid.UUID, driveId string) bool
	PhotoLikes(driveId string) ([]*Guest, error)
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

func (db *DB) DeleteLikeStore() error {
	_, err := db.Exec("DROP TABLE IF EXISTS likes;")
	return err
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

func (db *DB) DeleteLikes(uuid uuid.UUID) error {
	const stmt = "DELETE FROM likes WHERE guest = $1"
	if _, err := db.Exec(stmt, uuid.String()); err != nil {
		return err
	} else {
		return nil
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

func (db *DB) Like(guest uuid.UUID, driveId string) bool {
	const stmt = "SELECT 1 FROM likes WHERE guest = $1 AND driveId = $2"
	if rows, err := db.Query(stmt, guest.String(), driveId); err == nil {
		defer rows.Close()
		return rows.Next()
	} else {
		return false
	}
}

func (db *DB) PhotoLikes(driveId string) ([]*Guest, error) {
	//const stmt = "SELECT name,email FROM guests WHERE id IN (SELECT guest FROM likes WHERE driveId = $1)"
	const stmt = "select name, email FROM likes, guests WHERE driveId = $1 AND likes.guest = guests.id"
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
	return guests, nil
}
