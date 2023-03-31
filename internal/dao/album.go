package dao

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"strings"
)

type AlbumPG struct {
	db         *sqlx.DB
	fields     []string
	insertStmt string
	updateStmt string
}

func NewAlbumPG(db *sqlx.DB) *AlbumPG {
	fields := getStructFields(&Album{})
	return &AlbumPG{db, fields,
		buildInsertNamed("album", fields),
		buildUpdateNamed2("album", fields, "id", "id")}
}

func (dao *AlbumPG) Add(name, description, coverpic string) (*Album, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("white space/empty names not allowed")
	}
	album := Album{Id: uuid.New(), Name: name, Description: description, CoverPic: coverpic}
	_, err := dao.db.NamedExec(dao.insertStmt, &album)
	return &album, err
}

func (dao *AlbumPG) Get(id uuid.UUID) (*Album, error) {
	ret := Album{}
	err := dao.db.Get(&ret, "SELECT * FROM album WHERE id = $1", id)
	return &ret, err
}

func (dao *AlbumPG) List() ([]*Album, error) {
	ret := []*Album{}
	err := dao.db.Select(&ret, "SELECT * FROM album")
	return ret, err
}

func (dao *AlbumPG) Photos(id uuid.UUID, private bool) ([]*Photo, error) {
	ret := []*Photo{}
	stmt := "SELECT * FROM img WHERE private = false AND id IN (SELECT photoId FROM albumphotos WHERE albumId = $1)"
	if private {
		fmt.Println("executing private...")
		stmt = "SELECT * FROM img WHERE id IN (SELECT photoId FROM albumphotos WHERE albumId = $1)"
	}
	err := dao.db.Select(&ret, stmt, id)
	return ret, err
}

func (dao *AlbumPG) Delete(id uuid.UUID) error {

	if _, err := dao.db.Exec("DELETE FROM album WHERE id = $1", id); err == nil {
		_, err1 := dao.db.Exec("DELETE FROM albumphotos WHERE albumId = $1", id)
		return err1
	} else {
		return err
	}

	/*stmt := `
	DELETE FROM album WHERE id = $1;
	DELETE FROM albumphotos WHERE albumId = $1;
	`
		_, err := dao.db.Exec(stmt, id)
		return err*/

}

func (dao *AlbumPG) Has(id uuid.UUID) bool {
	return has(dao.db, "album", "id", id)
}

func (dao *AlbumPG) HasByName(name string) bool {
	return has(dao.db, "album", "name", name)
}

func (dao *AlbumPG) Albums(photoId uuid.UUID) ([]*Album, error) {
	ret := []*Album{}
	stmt := "SELECT * FROM album WHERE id IN (select albumId FROM albumphotos WHERE photoId = $1)"
	err := dao.db.Select(&ret, stmt, photoId)
	return ret, err
}

func (dao *AlbumPG) Update(album *Album) (*Album, error) {
	if _, err := dao.db.NamedExec(dao.updateStmt, album); err != nil {
		return nil, err
	} else {
		return dao.Get(album.Id)
	}

}

func (dao *AlbumPG) UpdatePhoto(albumIds []uuid.UUID, photoId uuid.UUID) error {
	//check img
	if !has(dao.db, "img", "id", photoId) {
		return fmt.Errorf("photoId does not exist")
	}

	//check album Ids
	for _, id := range albumIds {
		if !dao.Has(id) {
			return fmt.Errorf("non existent album")
		}
	}
	if _, err := dao.db.Exec("DELETE FROM albumphotos WHERE photoId = $1", photoId); err != nil {
		return err
	}

	const addAlbumPhoto = "INSERT INTO albumphotos (albumId, photoId) VALUES ($1, $2)"
	for _, a := range albumIds {
		if _, err := dao.db.Exec(addAlbumPhoto, a, photoId); err != nil {
			return nil
		}
	}
	return nil

}
