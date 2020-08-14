package model

import "go.uber.org/zap"

type AlbumPhotoStore interface {
	AddAlbumPhoto(album string, photoId string) error
	CreateAlbumPhotoStore() error
	//DeleteAlbumPhotoAlbum(album string) error
	//DeleteAlbumPhotoPhoto(photo string) error
	DeleteAlbumPhotoStore() error
	HasAlbumPhoto(album string, photoId string) bool
	UpdatePhotoAlbums(albums []string, photoId string) error
}

func (db *DB) AddAlbumPhoto(album string, photoId string) error {
	const stmt = "INSERT INTO albumphoto (album, driveId) VALUES ($1, $2)"
	_, err := db.Exec(stmt, album, photoId)
	return err
}

func (db *DB) CreateAlbumPhotoStore() error {
	const stmt = `
	CREATE TABLE IF NOT EXISTS albumphoto (
		album TEXT,
		driveId TEXT,
		PRIMARY KEY (album, driveId)
	);
`
	_, err := db.Exec(stmt)
	return err
}

/*
func (db *DB) DeleteAlbumPhotoAlbum(album string) error {
	const stmt = "DELETE FROM albumphoto WHERE album = $1"
	_, err := db.Exec(stmt, album);
	return err
}
func (db *DB) DeleteAlbumPhotoPhoto(driveId string) error {
	const stmt = "DELETE FROM albumphoto WHERE driveId = $1"
	_, err := db.Exec(stmt, driveId);
	return err
}
*/

func (db *DB) DeleteAlbumPhotoStore() error {
	_, err := db.Exec("DROP TABLE IF EXISTS albumphoto;")
	return err
}

func (db *DB) HasAlbumPhoto(album, photoId string) bool {
	const stmt = "SELECT 1 FROM albumphoto WHERE album = $1 AND driveId = $2"
	if rows, err := db.Query(stmt, trim(album), photoId); err == nil {
		defer rows.Close()
		return rows.Next()
	} else {
		logger.Errorw("could not check albumphoto", zap.Error(err))
		return false
	}
}

func (db *DB) UpdatePhotoAlbums(album []string, photoId string) error {
	//delete all old albums
	if _, err := db.Exec("DELETE FROM albumphoto WHERE driveId = $1", photoId); err != nil {
		return err
	}
	for _, a := range album {
		if err := db.AddAlbumPhoto(a, photoId); err != nil {
			return err
		}
	}
	return nil
}
