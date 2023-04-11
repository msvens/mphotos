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
	//select img.id FROM img JOIN albumphotos ap ON img.id = ap.photoid WHERE ap.albumid = '1035a8c9-72e9-4f77-ae79-afbe80fc458c' AND img.private = false order by ap.photoorder NULLS LAST
	stmt := "SELECT img.* FROM img JOIN albumphotos ap ON img.id = ap.photoid WHERE ap.albumid = $1 AND img.private = false ORDER BY ap.photoorder NULLS LAST"
	if private {
		stmt = "SELECT img.* FROM img JOIN albumphotos ap ON img.id = ap.photoid WHERE ap.albumid = $1 ORDER BY ap.photoorder NULLS LAST"
	}
	err := dao.db.Select(&ret, stmt, id)
	return ret, err
}

/*
func (dao *AlbumPG) PhotosOld(id uuid.UUID, private bool) ([]*Photo, error) {
	ret := []*Photo{}
	stmt := "SELECT * FROM img WHERE private = false AND id IN (SELECT photoId FROM albumphotos WHERE albumId = $1)"
	if private {
		stmt = "SELECT * FROM img WHERE id IN (SELECT photoId FROM albumphotos WHERE albumId = $1)"
	}
	err := dao.db.Select(&ret, stmt, id)
	return ret, err
}
*/

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

func (dao *AlbumPG) GetOrder(albumId uuid.UUID) ([]uuid.UUID, error) {
	ret := []uuid.UUID{}
	stmt := "SELECT photoid FROM albumphotos WHERE albumId = $1 AND photoorder IS NOT NULL ORDER BY photoorder"
	err := dao.db.Select(&ret, stmt, albumId)
	return ret, err
}

func (dao *AlbumPG) Update(album *Album) (*Album, error) {
	if _, err := dao.db.NamedExec(dao.updateStmt, album); err != nil {
		return nil, err
	} else {
		return dao.Get(album.Id)
	}

}

func (dao *AlbumPG) UpdateOrder(id uuid.UUID, photoIds []uuid.UUID) (*Album, error) {

	stmt := `
        UPDATE albumPhotos
        SET photoOrder = new_order
        FROM unnest($2::uuid[], $3::int[]) AS updates(id, new_order)
        WHERE albumPhotos.albumId = $1 AND albumPhotos.photoId = updates.id
    `
	orders := make([]int, len(photoIds), len(photoIds))

	for i := range orders {
		orders[i] = i + 1
	}
	if _, err := dao.db.Exec(stmt, id, photoIds, orders); err != nil {
		return nil, err
	} else {
		return dao.Get(id)
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
