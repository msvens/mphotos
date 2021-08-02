package model

import (
	"encoding/json"
	"fmt"
)

type User struct {
	Name            string `json:"name"`
	Bio             string `json:"bio"`
	Pic             string `json:"pic"`
	DriveFolderId   string `json:"driveFolderId,omitempty"`
	DriveFolderName string `json:"driveFolderName,omitempty"`
}

type UserStore interface {
	CreateUserStore() error
	DeleteUserStore() error
	UpdateUser(u *User) (*User, error)
	UpdateUserConfig(c string) error
	User() (*User, error)
	UserConfig() (string, error)
}

func (db *DB) CreateUserStore() error {
	const stmt = `
CREATE TABLE IF NOT EXISTS users (
	id INT PRIMARY KEY,
	name TEXT NOT NULL,
	bio TEXT NOT NULL,
	pic TEXT NOT NULL,
	driveFolderId TEXT NOT NULL,
	driveFolderName TEXT NOT NULL,
	config TEXT NOT NULL
);
INSERT INTO users (id, name, bio, pic, driveFolderId, driveFolderName, config) VALUES (23657, '', '', '', '','','{}') ON CONFLICT (id) DO NOTHING;
`
	_, err := db.Exec(stmt)
	return err
}

func (db *DB) DeleteUserStore() error {
	_, err := db.Exec("DROP TABLE IF EXISTS users;")
	return err
}

func (db *DB) User() (*User, error) {
	const stmt = "SELECT name,bio,pic,driveFolderId,driveFolderName FROM users LIMIT 1"
	resp := User{}
	r := db.QueryRow(stmt)
	if err := r.Scan(&resp.Name, &resp.Bio, &resp.Pic, &resp.DriveFolderId, &resp.DriveFolderName); err != nil {
		return nil, err
	}
	return &resp, nil
}

//Todo: Make this update more clear (i.e. cannot update drive relevant stuff
func (db *DB) UpdateUser(u *User) (*User, error) {
	const stmt = "UPDATE users SET (name, bio, pic) = ($1, $2, $3)"

	if _, err := db.Exec(stmt, u.Name, u.Bio, u.Pic); err != nil {
		return nil, err
	}
	return db.User()
}

func (db *DB) UserConfig() (string, error) {
	const stmt = "SELECT config FROM users LIMIT 1"
	var c string
	r := db.QueryRow(stmt)
	if err := r.Scan(&c); err != nil {
		return c, err
	}
	return c, nil
}

func (db *DB) UpdateUserConfig(c string) error {
	if !json.Valid([]byte(c)) {
		return fmt.Errorf("Non valid json config: %s", c)
	}
	const stmt = "UPDATE users SET config = $1"
	_, err := db.Exec(stmt, c)
	return err
}
