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

func (dao *AlbumPG) AddPhotos(id uuid.UUID, photoIds []uuid.UUID) (int, error) {
	//first get album
	if !dao.Has(id) {
		return 0, fmt.Errorf("Could not find album")
	}

	//check if all images exist
	query, args, err := sqlx.In("SELECT COUNT(*) FROM img WHERE id IN (?)", photoIds)
	if err != nil {
		return 0, err
	}
	query = dao.db.Rebind(query)
	var count int
	err = dao.db.QueryRowx(query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}
	if count != len(photoIds) {
		return 0, fmt.Errorf("Missing photos")
	}

	//now insert images one after the other (its just too messy to do it in a single
	//insert statement using sqlx and very little performance improvements...
	insStmt := "INSERT INTO albumphotos (albumId, photoId) VALUES ($1, $2) ON CONFLICT DO NOTHING"
	var numRows int64
	for _, pid := range photoIds {
		if res, err := dao.db.Exec(insStmt, id, pid); err == nil {
			r, _ := res.RowsAffected()
			numRows += r
		} else {
			return int(numRows), err
		}
	}
	return int(numRows), nil
}

func (dao *AlbumPG) ClearPhotos(id uuid.UUID) (int, error) {
	if !dao.Has(id) {
		return 0, fmt.Errorf("Could not find album")
	}
	if res, err := dao.db.Exec("DELETE FROM albumphotos WHERE albumId = $1", id); err != nil {
		return 0, err
	} else {
		rows, _ := res.RowsAffected()
		return int(rows), nil
	}

}

func (dao *AlbumPG) DeletePhotos(id uuid.UUID, photoIds []uuid.UUID) (int, error) {
	if !dao.Has(id) {
		return 0, fmt.Errorf("Could not find album")
	}
	var affectedRows int64
	insStmt := "DELETE FROM albumphotos WHERE albumId = $1 AND photoId = $2"
	for _, pid := range photoIds {
		if r, err := dao.db.Exec(insStmt, id, pid); err == nil {
			affected, _ := r.RowsAffected()
			affectedRows += affected
		} else {
			return int(affectedRows), err
		}
	}
	return int(affectedRows), nil
}

func (dao *AlbumPG) SetPhotos(id uuid.UUID, photoIds []uuid.UUID) (int, error) {
	if _, err := dao.ClearPhotos(id); err != nil {
		return 0, err
	} else {
		return dao.AddPhotos(id, photoIds)
	}
}

func (dao *AlbumPG) Get(id uuid.UUID) (*Album, error) {
	ret := Album{}
	err := dao.db.Get(&ret, "SELECT * FROM album WHERE id = $1", id)
	return &ret, err
}

func (dao *AlbumPG) GetByName(name string) (*Album, error) {
	ret := Album{}
	err := dao.db.Get(&ret, "SELECT * FROM album WHERE name = $1", name)
	return &ret, err
}

func (dao *AlbumPG) List() ([]*Album, error) {
	ret := []*Album{}
	err := dao.db.Select(&ret, "SELECT * FROM album")
	return ret, err
}

func (dao *AlbumPG) Photos(id uuid.UUID) ([]*Photo, error) {
	if !dao.Has(id) {
		return nil, fmt.Errorf("No such album")
	}
	stmt := "SELECT img.* FROM img JOIN albumphotos ap ON img.id = ap.photoid WHERE ap.albumid = $1"
	ret := []*Photo{}
	err := dao.db.Select(&ret, stmt, id)
	return ret, err
}

func (dao *AlbumPG) SelectPhotos(id uuid.UUID, filter PhotoFilter, r Range, order PhotoOrder) ([]*Photo, error) {
	var stmt strings.Builder
	stmt.WriteString("SELECT img.* FROM img JOIN albumphotos ap ON img.id = ap.photoid")

	//where clause
	if filter.CameraModel == "" {
		stmt.WriteString(" WHERE ap.albumid = $1")
	} else {
		stmt.WriteString(" WHERE ap.albumid = $1 AND img.cameramodel = $2")
	}
	//order by
	switch order {
	case UploadDate:
		stmt.WriteString(" ORDER BY img.uploaddate DESC")
	case OriginalDate:
		stmt.WriteString(" ORDER BY img.originaldate DESC")
	case ManualOrder:
		stmt.WriteString(" ORDER BY ap.photoorder NULLS LAST")
	}
	//limit
	if r.Limit > 0 {
		fmt.Fprintf(&stmt, " LIMIT %d OFFSET %d", r.Limit, r.Offset)
	}

	ret := []*Photo{}
	var err error
	if filter.CameraModel == "" {
		err = dao.db.Select(&ret, stmt.String(), id)
	} else {
		err = dao.db.Select(&ret, stmt.String(), id, filter.CameraModel)
	}
	return ret, err
}

/*
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

*/

func (dao *AlbumPG) Delete(id uuid.UUID) error {

	if _, err := dao.db.Exec("DELETE FROM album WHERE id = $1", id); err == nil {
		_, err1 := dao.db.Exec("DELETE FROM albumphotos WHERE albumId = $1", id)
		return err1
	} else {
		return err
	}

}

func (dao *AlbumPG) Has(id uuid.UUID) bool {
	return has(dao.db, "album", "id", id)
}

func (dao *AlbumPG) HasByName(name string) bool {
	return has(dao.db, "album", "name", name)
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
