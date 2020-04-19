package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	_ "github.com/lib/pq"
	"github.com/msvens/mexif"
	"github.com/msvens/mphotos/config"
)

type DbService struct {
	Db *sql.DB
}

var NoSuchPhoto = errors.New("No such Photo Id")

func NewDbService() (*DbService, error) {
	var dbs DbService
	if err := dbs.Connect(); err != nil {
		return nil, err
	} else {
		return &dbs, nil
	}
}

func (dbs *DbService) Connect() error {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.DbHost(), config.DbPort(), config.DbUser(), config.DbPassword(), config.DbName())
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return err
	}
	dbs.Db = db
	return nil
}

func (dbs *DbService) CreateTables() error {
	if _, err := dbs.Db.Exec(createPhotoTable); err != nil {
		return err
	}
	if _, err := dbs.Db.Exec(createExifTable); err != nil {
		return err
	}
	if _, err := dbs.Db.Exec(createUserTable); err != nil {
		return err
	}
	return nil
}

func (dbs *DbService) DropTables() error {
	if _, err := dbs.Db.Exec(dropPhotoTable); err != nil {
		return err
	}
	if _, err := dbs.Db.Exec(dropExifTable); err != nil {
		return err
	}
	if _, err := dbs.Db.Exec(dropUserTable); err != nil {
		return err
	}
	return nil
}

func (dbs *DbService) Close() error {
	return dbs.Db.Close()
}



func (dbs *DbService) AddPhoto(p *Photo,exif *mexif.ExifCompact) error {

	_, err := dbs.Db.Exec(insPhotoStmt, p.DriveId, p.Md5, p.FileName, p.Title, p.Keywords, p.Description, p.DriveDate, p.OriginalDate,
		p.CameraMake, p.CameraModel, p.LensMake, p.LensModel, p.Iso, p.FNumber, p.Exposure)
	if err != nil {
		return err
	}
	data, err := json.Marshal(exif);
	if err != nil {
		return err
	}
	_, err = dbs.Db.Exec(insExifStmt,p.DriveId, string(data))
	return err;
}

func (dbs *DbService) Contains(driveId string) bool {
	if rows, err := dbs.Db.Query(containsIdStmt, driveId); err == nil {
		return rows.Next()
	} else {
		return false
	}
}

func (dbs *DbService) GetExif(driveId string) (*Exif, error) {
	resp := Exif{Data: &mexif.ExifCompact{}}
	var data string
	if err := dbs.Db.QueryRow(getExifStmt, driveId).Scan(&resp.DriveId, &data); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(data), resp.Data); err != nil {
		return nil, err
	}
	return &resp, nil

}

func (dbs *DbService) GetId(driveId string) (*Photo, error) {
	resp := Photo{}
	r :=dbs.Db.QueryRow(getIdStmt, driveId)
	if err := scanRow(&resp, r); err != nil {
		return nil, err
	} else {
		return &resp, nil
	}
}

func (dbs *DbService) List() ([]*Photo, error) {
	rows, err := dbs.Db.Query(getStmt)
	var p *Photo
	if err != nil {
		return nil, err
	}

	var photos []*Photo
	for ; rows.Next(); {
		p = &Photo{}
		if err = scanRows(p, rows); err == nil {
			photos = append(photos, p)
		} else {
			return nil, err
		}
	}
	return photos, nil
}

func scanRows(p *Photo, r *sql.Rows) error {
	err := r.Scan(&p.DriveId, &p.Md5, &p.FileName, &p.Title, &p.Keywords, &p.Description, &p.DriveDate,
		&p.OriginalDate, &p.CameraMake, &p.CameraModel, &p.LensMake, &p.LensModel,
		&p.Iso, &p.FNumber, &p.Exposure)

	switch err {
	case sql.ErrNoRows:
		return NoSuchPhoto
	case nil:
		return nil
	default:
		return err
	}
}

func scanRow(p *Photo, r *sql.Row) error {
	err := r.Scan(&p.DriveId, &p.Md5, &p.FileName, &p.Title, &p.Keywords, &p.Description, &p.DriveDate,
		&p.OriginalDate, &p.CameraMake, &p.CameraModel, &p.LensMake, &p.LensModel,
		&p.Iso, &p.FNumber, &p.Exposure)

	switch err {
	case sql.ErrNoRows:
		return NoSuchPhoto
	case nil:
		return nil
	default:
		return err
	}
}



