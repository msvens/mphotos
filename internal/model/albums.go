package model

import (
	"fmt"
	"go.uber.org/zap"
)

type Album struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CoverPic    string `json:"coverPic"`
}

func (a Album) String() string {
	return fmt.Sprintf("{Id: %v, Name: %s, Description: %s, CoverPic: %s}", a.Id, a.Name, a.Description, a.CoverPic)
}

type AlbumStore interface {
	AddAlbum(name, description, coverpic string) (*Album, error)
	Album(id int) (*Album, error)
	Albums() ([]*Album, error)
	AlbumPhotos(albumId int, filter PhotoFilter) ([]*Photo, error)
	//CameraAlbums() ([]*Album, error)
	//CameraAlbum(cameraModel string) (*Album, error)
	CreateAlbumStore() error
	DeleteAlbum(id int) error
	DeleteAlbumStore() error
	HasAlbum(id int) bool
	HasAlbumName(name string) bool
	PhotoAlbums(photoId string) ([]*Album, error)
	UpdateAlbum(album *Album) (*Album, error)
	UpdatePhotoAlbums(albumIds []int, photoId string) error
}

func (db *DB) AddAlbum(name, description, coverpic string) (*Album, error) {
	if trim(name) == "" {
		return nil, fmt.Errorf("white space/empty names not allowed")
	}
	const stmt = "INSERT INTO albums (name, description, coverPic) VALUES ($1, $2, $3) RETURNING id;"
	var id int
	if err := db.QueryRow(stmt, name, description, coverpic).Scan(&id); err != nil {
		return nil, err
	} else {
		return &Album{Id: id, Name: name, Description: description, CoverPic: coverpic}, nil
	}
}

func (db *DB) Album(id int) (*Album, error) {
	const stmt = "SELECT id,name,description,coverPic FROM albums WHERE id = $1"
	resp := Album{}
	if err := db.QueryRow(stmt, id).Scan(&resp.Id, &resp.Name, &resp.Description, &resp.CoverPic); err != nil {
		return nil, err
	} else {
		return &resp, nil
	}
}

func (db *DB) CreateAlbumStore() error {
	const stmt = `
	CREATE TABLE IF NOT EXISTS albums (
	    id SERIAL PRIMARY KEY,
		name TEXT,
		description TEXT NOT NULL,
		coverPic TEXT NOT NULL,
		CONSTRAINT albumname UNIQUE (name)
	);
	CREATE TABLE IF NOT EXISTS albumphoto (
		albumId INTEGER,
		driveId TEXT,
		PRIMARY KEY (albumId, driveId)
	);
`
	_, err := db.Exec(stmt)
	return err
}

func (db *DB) DeleteAlbum(id int) error {
	const delAlbumStmt = "DELETE FROM albums WHERE id = $1"
	const delAlbumPhotoStmt = "DELETE FROM albumphoto WHERE albumId = $1"
	if _, err := db.Exec(delAlbumStmt, id); err != nil {
		return err
	}
	_, err := db.Exec(delAlbumPhotoStmt, id)
	return err
}

func (db *DB) DeleteAlbumStore() error {
	const stmt = `
	DROP TABLE IF EXISTS albumphoto;
	DROP TABLE IF EXISTS albums;
`
	_, err := db.Exec(stmt)
	return err
}

func (db *DB) HasAlbum(id int) bool {
	const stmt = "SELECT 1 FROM albums WHERE id = $1"
	if rows, err := db.Query(stmt, id); err == nil {
		defer rows.Close()
		return rows.Next()
	} else {
		logger.Errorw("could not check album", zap.Error(err))
		return false
	}
}

func (db *DB) HasAlbumName(name string) bool {
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
	const stmt = "SELECT id,name,description,coverPic FROM albums"
	albums := []*Album{}
	if rows, err := db.Query(stmt); err != nil {
		return nil, err
	} else {
		defer rows.Close()
		for rows.Next() {
			var album = Album{}
			if err := rows.Scan(&album.Id, &album.Name, &album.Description, &album.CoverPic); err != nil {
				return nil, err
			}
			albums = append(albums, &album)
		}
	}
	return albums, nil
}

func (db *DB) AlbumPhotos(albumId int, filter PhotoFilter) ([]*Photo, error) {
	var stmt string
	if !filter.Private {
		stmt = "SELECT " + photoCols + " FROM photos WHERE private = false AND driveId IN (SELECT driveId FROM albumphoto WHERE albumId = $1)"
	} else {
		stmt = "SELECT " + photoCols + " FROM photos WHERE driveId IN (SELECT driveId FROM albumphoto WHERE albumId = $1)"
	}
	if rows, err := db.Query(stmt, albumId); err != nil {
		return nil, err
	} else {
		defer rows.Close()
		return scanPhotos(rows)
	}
}

func (db *DB) PhotoAlbums(photoId string) ([]*Album, error) {
	const stmt = "SELECT id, name, description, coverPic FROM albums WHERE id IN (SELECT albumId FROM albumphoto WHERE driveId = $1)"
	albums := []*Album{}
	if rows, err := db.Query(stmt, photoId); err != nil {
		return nil, err
	} else {
		defer rows.Close()
		for rows.Next() {
			var album = Album{}
			if err := rows.Scan(&album.Id, &album.Name, &album.Description, &album.CoverPic); err != nil {
				return nil, err
			}
			albums = append(albums, &album)
		}
	}
	return albums, nil
}

func (db *DB) UpdateAlbum(album *Album) (*Album, error) {
	const stmt = "UPDATE albums SET (name, description, coverPic) = ($1, $2, $3) WHERE id = $4"
	if _, err := db.Exec(stmt, album.Name, album.Description, album.CoverPic, album.Id); err != nil {
		return nil, err
	}
	return db.Album(album.Id)
}

func (db *DB) UpdatePhotoAlbums(album []int, photoId string) error {
	//first check if photo exists (this should be handled better as a photo could be deleted between statements
	if !db.HasPhoto(photoId, true) {
		return fmt.Errorf("non existent photo")
	}
	//check album Ids
	for _, id := range album {
		if !db.HasAlbum(id) {
			return fmt.Errorf("non existent album")
		}
	}
	//delete all old albums
	if _, err := db.Exec("DELETE FROM albumphoto WHERE driveId = $1", photoId); err != nil {
		return err
	}
	const addAlbumPhoto = "INSERT INTO albumphoto (albumId, driveId) VALUES ($1, $2)"
	for _, a := range album {
		if _, err := db.Exec(addAlbumPhoto, a, photoId); err != nil {
			return nil
		}
	}
	return nil
}
