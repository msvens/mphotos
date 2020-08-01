package model

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/msvens/mexif"
	"time"
)

type PhotoStore interface {
	AddPhoto(p *Photo, exif *mexif.ExifCompact) error
	CreatePhotoStore() error
	DeletePhoto(id string) (bool, error)
	DeletePhotoStore() error
	Exif(id string) (*Exif, error)
	HasPhoto(id string, private bool) bool
	Photo(id string, private bool) (*Photo, error)
	Photos(r Range, order PhotoOrder, filter PhotoFilter) ([]*Photo, error)
	SetPrivatePhoto(private bool, id string) (*Photo, error)
	UpdatePhoto(title string, description string, keywords []string, id string) (*Photo, error)
}

type PhotoOrder int

const (
	None PhotoOrder = iota
	DriveDate
	OriginalDate
)

const photoCols = "driveId,md5,fileName,title,keywords,description,driveDate,originalDate," +
	"cameraMake,cameraModel,lensMake,lensModel,focalLength,focalLength35,iso,fNumber,exposure," +
	"width,height,private,likes"

type Exif struct {
	DriveId string
	Data    *mexif.ExifCompact
}

type Photo struct {
	DriveId      string    `json:"driveId"`
	Md5          string    `json:"md5"`
	FileName     string    `json:"fileName"`
	Title        string    `json:"title"`
	Keywords     string    `json:"keywords"`
	Description  string    `json:"description"`
	DriveDate    time.Time `json:"driveDate"`
	OriginalDate time.Time `json:"originalDate"`

	CameraMake    string `json:"cameraMake"`
	CameraModel   string `json:"cameraModel"`
	LensMake      string `json:"lensMake,omitempty"`
	LensModel     string `json:"lensModel,omitempty"`
	FocalLength   string `json:"focalLength"`
	FocalLength35 string `json:"focalLength35"`

	Iso      uint    `json:"iso"`
	Exposure string  `json:"exposure"`
	FNumber  float32 `json:"fNumber"`
	Width    uint    `json:"width"`
	Height   uint    `json:"height"`
	Private  bool    `json:"private"`
	Likes    uint    `json:"likes"`
}

type PhotoFilter struct {
	Private     bool
	CameraModel string
}

type Range struct {
	Offset int
	Limit  int
}

var NoSuchPhoto = errors.New("No such Photo Id")

func (db *DB) AddPhoto(p *Photo, exif *mexif.ExifCompact) error {
	const insPhoto = "INSERT INTO photos (" + photoCols + ") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21);"
	const insExif = "INSERT INTO exif (driveId, data) VALUES ($1, $2)"

	fmt.Println(insPhoto)
	_, err := db.Exec(insPhoto, p.DriveId, p.Md5, p.FileName, p.Title, p.Keywords, p.Description, p.DriveDate, p.OriginalDate,
		p.CameraMake, p.CameraModel, p.LensMake, p.LensModel, p.FocalLength, p.FocalLength35, p.Iso,
		p.FNumber, p.Exposure, p.Width, p.Height, p.Private, p.Likes)
	if err != nil {
		return err
	}

	fmt.Println("added photo")
	//insert extended exif information
	data, err := json.Marshal(exif)
	if err != nil {
		return err
	}
	_, err = db.Exec(insExif, p.DriveId, string(data))
	return err
}

func (db *DB) CreatePhotoStore() error {
	const stmt = `
CREATE TABLE IF NOT EXISTS photos (
	driveId TEXT PRIMARY KEY,
	md5 TEXT NOT NULL,
	fileName TEXT NOT NULL,
	title TEXT NOT NULL,
	keywords TEXT,
	description TEXT,
	driveDate TIMESTAMP NOT NULL,
	originalDate TIMESTAMP NOT NULL,
	cameraMake TEXT NOT NULL,
	cameraModel TEXT NOT NULL,
	lensMake TEXT,
	lensModel TEXT,
	focalLength TEXT,
	focalLength35 TEXT,
 	iso INTEGER NOT NULL,
	fNumber REAL NOT NULL,
	exposure TEXT NOT NULL,
	width INTEGER NOT NULL,
	height INTEGER NOT NULL,
	private BOOLEAN NOT NULL,
	likes INTEGER NOT NULL
);`
	if _, err := db.Exec(stmt); err != nil {
		return err
	}
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS exif (driveId TEXT PRIMARY KEY,data TEXT NOT NULL);")
	return err

}

func (db *DB) DeletePhoto(id string) (bool, error) {
	const delPhoto = "DELETE FROM photos WHERE driveId = $1;"
	const delExif = "DELETE FROM exif WHERE driveId = $1"
	if _, err := db.Exec(delPhoto, id); err != nil {
		return false, err
	}
	if res, err := db.Exec(delExif, id); err != nil {
		return false, err
	} else {
		cnt, _ := res.RowsAffected()
		return cnt > 0, nil
	}
}

func (db *DB) DeletePhotoStore() error {
	if _, err := db.Exec("DROP TABLE IF EXISTS photos;"); err != nil {
		return err
	}
	_, err := db.Exec("DROP TABLE IF EXISTS exif;")
	return err
}

func (db *DB) Exif(id string) (*Exif, error) {
	const stmt = "SELECT * FROM exif WHERE driveId = $1"
	resp := Exif{Data: &mexif.ExifCompact{}}
	var data string
	if err := db.QueryRow(stmt, id).Scan(&resp.DriveId, &data); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(data), resp.Data); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (db *DB) HasPhoto(id string, private bool) bool {
	var stmt string
	if !private {
		stmt = "SELECT 1 FROM photos WHERE driveId = $1 AND private = false"
	} else {
		stmt = "SELECT 1 FROM photos WHERE driveId = $1"
	}
	if rows, err := db.Query(stmt, id); err == nil {
		defer rows.Close()
		return rows.Next()
	} else {
		return false
	}
}

func (db *DB) Photo(id string, private bool) (*Photo, error) {
	var stmt = "SELECT " + photoCols + " FROM photos WHERE private = false AND driveId = $1"
	if private {
		stmt = "SELECT " + photoCols + " FROM photos WHERE driveId = $1"
	}
	if rows, err := db.Query(stmt, id); err != nil {
		return nil, err
	} else {
		defer rows.Close()
		return scanNextPhoto(rows)
	}
}

func (db *DB) Photos(r Range, order PhotoOrder, filter PhotoFilter) ([]*Photo, error) {

	stmt := "SELECT " + photoCols + " FROM photos"
	//check filter
	if !filter.Private {
		stmt += " WHERE private = false"
		if filter.CameraModel != "" {
			stmt += " AND cameraModel = $1"
		}
	} else if filter.CameraModel != "" {
		stmt += " WHERE cameraModel = $1"
	}

	//check order by
	switch order {
	case DriveDate:
		stmt += " ORDER BY driveDate DESC"
	case OriginalDate:
		stmt += " ORDER BY originalDate DESC"
	}
	//check limit:
	if r.Limit > 0 {
		stmt += fmt.Sprintf(" LIMIT %d OFFSET %d", r.Limit, r.Offset)
	}
	//logger.Debugw("getPhotos", "query", stmt)
	var rows *sql.Rows
	var err error
	if filter.CameraModel != "" {
		rows, err = db.Query(stmt, filter.CameraModel)
	} else {
		rows, err = db.Query(stmt)
	}
	if err != nil {
		return nil, err
	} else {
		defer rows.Close()
		return scanPhotos(rows)
	}
}

func (db *DB) SetPrivatePhoto(private bool, id string) (*Photo, error) {
	const stmt = "UPDATE photos SET private = $1 WHERE driveId = $2"
	if _, err := db.Exec(stmt, private, id); err != nil {
		return nil, err
	}
	return db.Photo(id, true)
}

func (db *DB) UpdatePhoto(title string, description string, keywords []string, id string) (*Photo, error) {
	const stmt = "UPDATE photos SET (title, description,keywords) = ($1, $2, $3) WHERE driveId = $4"
	if _, err := db.Exec(stmt, title, description, trimAndJoin(keywords), id); err != nil {
		return nil, err
	}
	return db.Photo(id, true)
}

func scanNextPhoto(rows *sql.Rows) (*Photo, error) {
	if rows.Next() {
		p := &Photo{}
		if err := scanPhoto(p, rows); err != nil {
			return nil, err
		}
		return p, nil

	} else {
		return nil, NoSuchPhoto
	}
}

func scanPhotos(rows *sql.Rows) ([]*Photo, error) {
	var p *Photo
	var photos []*Photo
	for rows.Next() {
		p = &Photo{}
		if err := scanPhoto(p, rows); err == nil {
			photos = append(photos, p)
		} else {
			return nil, err
		}
	}
	return photos, nil
}

func scanPhoto(p *Photo, r *sql.Rows) error {
	err := r.Scan(&p.DriveId, &p.Md5, &p.FileName, &p.Title, &p.Keywords, &p.Description, &p.DriveDate,
		&p.OriginalDate, &p.CameraMake, &p.CameraModel, &p.LensMake, &p.LensModel, &p.FocalLength, &p.FocalLength35,
		&p.Iso, &p.FNumber, &p.Exposure, &p.Width, &p.Height, &p.Private, &p.Likes)

	switch err {
	case sql.ErrNoRows:
		return NoSuchPhoto
	case nil:
		return nil
	default:
		return err
	}
}
