package dao

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/msvens/mimage/metadata"
	"strings"
)

type PhotoPG struct {
	db              *sqlx.DB
	photoFields     []string
	insertIntoPhoto string
}

func NewPhotoPG(db *sqlx.DB) *PhotoPG {
	p := &Photo{}
	fields := getStructFields(p)
	return &PhotoPG{db, fields, buildInsertNamed("img", fields)}
}

func (dao *PhotoPG) Add(p *Photo, exif *metadata.Summary) error {
	if p.Id == uuid.Nil {
		p.Id = uuid.New()
	}
	if _, err := dao.db.NamedExec(dao.insertIntoPhoto, p); err != nil {
		return err
	}
	data, err := json.Marshal(exif)
	if err != nil {
		return err
	}
	_, err = dao.db.Exec("INSERT INTO exifdata (id,data) VALUES ($1, $2)", p.Id, string(data))
	return err
}

func (dao *PhotoPG) Albums(id uuid.UUID) ([]*Album, error) {
	if !dao.Has(id) {
		return nil, fmt.Errorf("no such photo")
	}
	ret := []*Album{}
	stmt := "SELECT * FROM album WHERE id IN (select albumId FROM albumphotos WHERE photoId = $1)"
	err := dao.db.Select(&ret, stmt, id)
	return ret, err
}

func (dao *PhotoPG) AddAlbums(id uuid.UUID, albumIds []uuid.UUID) (int, error) {
	if !dao.Has(id) {
		return 0, fmt.Errorf("could not find photo")
	}

	//check if all albums
	query, args, err := sqlx.In("SELECT COUNT(*) FROM album WHERE id IN (?)", albumIds)
	if err != nil {
		return 0, err
	}
	query = dao.db.Rebind(query)
	var count int
	err = dao.db.QueryRowx(query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}
	if count != len(albumIds) {
		return 0, fmt.Errorf("missing photos")
	}

	//now insert images
	var added int64
	insStmt := "INSERT INTO albumphotos (albumId, photoId) VALUES ($1, $2) ON CONFLICT DO NOTHING"
	//now insert images
	for _, aid := range albumIds {
		if res, err := dao.db.Exec(insStmt, aid, id); err == nil {
			r, _ := res.RowsAffected()
			added += r
		} else {
			return int(added), err
		}
	}
	return int(added), nil
}

func (dao *PhotoPG) ClearAlbums(id uuid.UUID) (int, error) {
	if !dao.Has(id) {
		return 0, fmt.Errorf("could not find photo")
	}
	if res, err := dao.db.Exec("DELETE FROM albumphotos WHERE photoId = $1", id); err != nil {
		return 0, err
	} else {
		rows, _ := res.RowsAffected()
		return int(rows), nil
	}
}

func (dao *PhotoPG) DeleteAlbums(id uuid.UUID, albumIds []uuid.UUID) (int, error) {
	if !dao.Has(id) {
		return 0, fmt.Errorf("could not find photo")
	}
	var deleted int64
	delStmt := "DELETE FROM albumphotos WHERE albumId = $1 AND photoId = $2"
	for _, aid := range albumIds {
		if res, err := dao.db.Exec(delStmt, aid, id); err == nil {
			r, _ := res.RowsAffected()
			deleted += r
		} else {
			return int(deleted), err
		}
	}
	return int(deleted), nil
}

func (dao *PhotoPG) Delete(id uuid.UUID) (bool, error) {

	deleted := false
	if res, err := dao.db.Exec("DELETE FROM img WHERE id = $1", id); err != nil {
		return false, err
	} else {
		cnt, _ := res.RowsAffected()
		deleted = cnt > 0
	}
	if _, err := dao.db.Exec("DELETE FROM exifData WHERE id = $1", id); err != nil {
		return deleted, err
	}
	if deleted {
		if _, err := dao.db.Exec("DELETE from reaction WHERE photoId = $1", id); err != nil {
			return deleted, err
		}
		if _, err := dao.db.Exec("DELETE from comment WHERE photoId = $1", id); err != nil {
			return deleted, err
		}
		if _, err := dao.db.Exec("DELETE from albumphotos WHERE photoId = $1", id); err != nil {
			return deleted, err
		}

	}
	return deleted, nil
}

func (dao *PhotoPG) Exif(id uuid.UUID) (*Exif, error) {
	var data string
	if err := dao.db.QueryRow("SELECT data FROM exifdata WHERE id = $1", id).Scan(&data); err != nil {
		return nil, err
	}
	resp := Exif{Id: id, Data: &metadata.Summary{}}
	if err := json.Unmarshal([]byte(data), resp.Data); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (dao *PhotoPG) Has(id uuid.UUID) bool {
	stmt := "SELECT 1 FROM img WHERE id = $1"
	if rows, err := dao.db.Query(stmt, id); err == nil {
		defer func() { _ = rows.Close() }()
		return rows.Next()
	} else {
		return false
	}
}

func (dao *PhotoPG) HasMd5(md5 string) bool {
	if rows, err := dao.db.Query("SELECT 1 FROM img WHERE md5 = $1", md5); err == nil {
		defer func() { _ = rows.Close() }()
		return rows.Next()
	} else {
		return false
	}
}

func (dao *PhotoPG) Get(id uuid.UUID) (*Photo, error) {
	ret := &Photo{}
	stmt := "SELECT * FROM img WHERE id = $1"
	err := dao.db.Get(ret, stmt, id)
	return ret, err
}

func (dao *PhotoPG) List() ([]*Photo, error) {
	ret := []*Photo{}
	err := dao.db.Select(&ret, "SELECT * FROM img ORDER BY uploaddate DESC")
	return ret, err
}

func (dao *PhotoPG) ListSource(source string) ([]*Photo, error) {
	ret := []*Photo{}
	err := dao.db.Select(&ret, "SELECT * from img WHERE source = $1", source)
	return ret, err
}

// Select returns photos honoring an optional album scope, an order and a page.
// filter.AlbumId (set server-side, never by a client) limits the result to that
// album's members — this is how the public photo list is confined to the
// photostream. A zero Range.Limit means "no limit" (return everything), which
// preserves the historic behavior of List.
func (dao *PhotoPG) Select(filter PhotoFilter, r Range, order PhotoOrder) ([]*Photo, error) {
	var stmt strings.Builder
	stmt.WriteString("SELECT * FROM img")

	var where []string
	var args []interface{}
	if filter.AlbumId != nil {
		args = append(args, *filter.AlbumId)
		where = append(where, fmt.Sprintf("id IN (SELECT photoid FROM albumphotos WHERE albumid = $%d)", len(args)))
	}
	// eq appends an "img.<col> = $n" predicate for each non-empty equipment
	// filter, keeping the placeholder index in step with the args slice.
	eq := func(col, val string) {
		if val != "" {
			args = append(args, val)
			where = append(where, fmt.Sprintf("img.%s = $%d", col, len(args)))
		}
	}
	eq("cameramodel", filter.CameraModel)
	eq("cameramake", filter.CameraMake)
	eq("lensmodel", filter.LensModel)
	eq("lensmake", filter.LensMake)
	if len(where) > 0 {
		stmt.WriteString(" WHERE ")
		stmt.WriteString(strings.Join(where, " AND "))
	}

	stmt.WriteString(orderByClause(order))

	if r.Limit > 0 {
		offset := r.Offset
		if offset < 0 {
			offset = 0
		}
		args = append(args, r.Limit, offset)
		fmt.Fprintf(&stmt, " LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	}

	ret := []*Photo{}
	err := dao.db.Select(&ret, stmt.String(), args...)
	return ret, err
}

// orderByClause maps a PhotoOrder to a stable ORDER BY with a deterministic
// tie-breaker on id, so LIMIT/OFFSET paging cannot drop or repeat rows across
// pages. The flat photo list has no album row-order, so None and ManualOrder
// fall back to upload date (the historic default of List).
func orderByClause(order PhotoOrder) string {
	switch order {
	case OriginalDate:
		return " ORDER BY originaldate DESC, id"
	default: // None, UploadDate, ManualOrder
		return " ORDER BY uploaddate DESC, id"
	}
}

func (dao *PhotoPG) Set(title string, description string, keywords []string, id uuid.UUID) (*Photo, error) {
	//join keywords
	var b strings.Builder
	for idx := 0; idx < len(keywords); idx++ {
		b.WriteString(strings.TrimSpace(keywords[idx]))
		if idx < len(keywords)-1 {
			b.WriteByte(',')
		}
	}
	stmt := "UPDATE img SET title = $1, description = $2, keywords = $3 WHERE id = $4"
	if _, err := dao.db.Exec(stmt, title, description, b.String(), id); err != nil {
		return nil, err
	}
	return dao.Get(id)
}

func (dao *PhotoPG) SetAlbums(id uuid.UUID, albumIds []uuid.UUID) (int, error) {
	if _, err := dao.ClearAlbums(id); err != nil {
		return 0, err
	} else {
		return dao.AddAlbums(id, albumIds)
	}
}
