package model

import (
	"go.uber.org/zap"
)

type Album struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CoverPic    string `json:"coverPic"`
}

type AlbumStore interface {
	AddAlbum(album *Album) error
	Album(name string) (*Album, error)
	Albums() ([]*Album, error)
	CreateAlbumStore() error
	DeleteAlbum(name string) error
	DeleteAlbumStore() error
	HasAlbum(name string) bool
	PhotoAlbums(id string) ([]*Album, error)
	UpdateAlbum(album *Album) (*Album, error)
}

func (db *DB) AddAlbum(album *Album) error {
	const stmt = "INSERT INTO albums (name, description, coverPic) VALUES ($1, $2, $3)"
	if _, err := db.Exec(stmt, album.Name, album.Description, album.CoverPic); err != nil {
		return err
	} else {
		return nil
	}
}

func (db *DB) Album(name string) (*Album, error) {
	const stmt = "SELECT name,description,coverPic FROM albums WHERE name = $1"
	resp := Album{}
	if err := db.QueryRow(stmt, name).Scan(&resp.Name, &resp.Description, &resp.CoverPic); err != nil {
		return nil, err
	} else {
		return &resp, nil
	}
}

func (db *DB) CreateAlbumStore() error {
	const stmt = `
	CREATE TABLE IF NOT EXISTS albums (
		name TEXT PRIMARY KEY,
		description TEXT NOT NULL,
		coverPic TEXT NOT NULL
	);
`
	_, err := db.Exec(stmt)
	return err
}

func (db *DB) DeleteAlbum(name string) error {
	const delAlbumStmt = "DELETE FROM albums WHERE name = $1"
	const delAlbumPhotoStmt = "DELETE FROM albumphoto WHERE album = $1"
	if _, err := db.Exec(delAlbumStmt, name); err != nil {
		return err
	}
	_, err := db.Exec(delAlbumPhotoStmt, name)
	return err

}

func (db *DB) DeleteAlbumStore() error {
	_, err := db.Exec("DROP TABLE IF EXISTS albums;")
	return err
}

func (db *DB) HasAlbum(name string) bool {
	const stmt = "SELECT 1 FROM albums WHERE name = $1"
	if rows, err := db.Query(stmt, name); err == nil {
		defer rows.Close()
		return rows.Next()
	} else {
		logger.Errorw("could not check album", zap.Error(err))
		return false
	}
}

func (db *DB) Albums() ([]*Album, error) {
	const stmt = "SELECT name,description,coverPic FROM albums"
	var albums []*Album
	if rows, err := db.Query(stmt); err != nil {
		return nil, err
	} else {
		defer rows.Close()
		for rows.Next() {
			var album = Album{}
			if err := rows.Scan(&album.Name, &album.Description, &album.CoverPic); err != nil {
				return nil, err
			}
			albums = append(albums, &album)
		}
	}
	return albums, nil
}

func (db *DB) PhotoAlbums(id string) ([]*Album, error) {
	const stmt = "SELECT name, description, coverPic FROM albums WHERE name IN (SELECT album FROM albumphoto WHERE driveId = $1)"
	var albums []*Album
	if rows, err := db.Query(stmt, id); err != nil {
		return nil, err
	} else {
		defer rows.Close()
		for rows.Next() {
			var album = Album{}
			if err := rows.Scan(&album.Name, &album.Description, &album.CoverPic); err != nil {
				return nil, err
			}
			albums = append(albums, &album)
		}
	}
	return albums, nil
}

func (db *DB) UpdateAlbum(album *Album) (*Album, error) {
	const stmt = "UPDATE albums SET (description, coverPic) = ($1, $2) WHERE name = $3"
	if _, err := db.Exec(stmt, album.Description, album.CoverPic, album.Name); err != nil {
		return nil, err
	}
	return db.Album(album.Name)
}
