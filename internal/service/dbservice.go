package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	_ "github.com/lib/pq"
	"github.com/msvens/mexif"
	"github.com/msvens/mphotos/internal/config"
	"go.uber.org/zap"
	"strings"
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
		logger.Errorw("could not connect to database", zap.Error(err))
		return err
	}
	err = db.Ping()
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
	if _, err := dbs.Db.Exec(createAlbumTable); err != nil {
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
	if _, err := dbs.Db.Exec(dropAlbumTable); err != nil {
		return err
	}
	return nil
}

func (dbs *DbService) Close() error {
	return dbs.Db.Close()
}

func (dbs *DbService) AddPhoto(p *Photo, exif *mexif.ExifCompact) error {

	_, err := dbs.Db.Exec(insPhotoStmt, p.DriveId, p.Md5, p.FileName, p.Title, p.Keywords, p.Description, p.DriveDate, p.OriginalDate,
		p.CameraMake, p.CameraModel, p.LensMake, p.LensModel, p.FocalLength, p.FocalLength35, p.Iso,
		p.FNumber, p.Exposure, p.Width, p.Height, p.Private, p.Album, p.Likes)
	if err != nil {
		return err
	}
	data, err := json.Marshal(exif)
	if err != nil {
		return err
	}
	_, err = dbs.Db.Exec(insExifStmt, p.DriveId, string(data))
	return err
}

func (dbs *DbService) AddAlbum(album *Album) error {
	if _, err := dbs.Db.Exec(insAlbumStmt, album.Name, album.Description, album.CoverPic); err != nil {
		return err
	} else {
		return nil
	}
}

func (dbs *DbService) Contains(driveId string) bool {
	if rows, err := dbs.Db.Query(containsIdStmt, driveId); err == nil {
		return rows.Next()
	} else {
		return false
	}
}

func (dbs *DbService) ContainsAlbum(name string) bool {
	if rows, err := dbs.Db.Query(containsAlbumStmt, name); err == nil {
		return rows.Next()
	} else {
		return false
	}
}

func (dbs *DbService) Delete(driveId string) (bool, error) {

	if _, err := dbs.Db.Exec(deleteStmt, driveId); err != nil {
		return false, err
	}
	if res, err := dbs.Db.Exec(deleteExifStmt, driveId); err != nil {
		return false, err
	} else {
		cnt, _ := res.RowsAffected()
		return cnt > 0, nil
	}
}

func (dbs *DbService) GetAlbum(name string) (*Album, error) {
	resp := Album{}
	if err := dbs.Db.QueryRow(getAlbumStmt, name).Scan(&resp.Name, &resp.Description, &resp.CoverPic); err != nil {
		return nil, err
	} else {
		return &resp, nil
	}
}

func (dbs *DbService) GetAlbums() ([]*Album, error) {
	var albums []*Album
	if rows, err := dbs.Db.Query(getAlbumsStmt); err != nil {
		return nil, err
	} else {
		for rows.Next() {
			var album = Album{}
			rows.Scan(&album.Name, &album.Description, &album.CoverPic)
			albums = append(albums, &album)
		}
	}
	return albums, nil
}

func (dbs *DbService) GetAlbumPhotos(name string, private bool) ([]*Photo, error) {
	var stmt = getAlbumPhotosPublicStmt
	if private {
		stmt = getAlbumPhotosStmt
	}
	nn := "%" + name + "%"
	if rows, err := dbs.Db.Query(stmt, nn); err != nil {
		return nil, err
	} else {
		return scanR(rows)
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

func (dbs *DbService) GetId(driveId string, private bool) (*Photo, error) {
	resp := Photo{}
	var stmt = getIdStmtPublic
	if private {
		stmt = getIdStmt
	}
	r := dbs.Db.QueryRow(stmt, driveId)
	if err := scanRow(&resp, r); err != nil {
		return nil, err
	} else {
		return &resp, nil
	}
}

func (dbs *DbService) GetUser() (*User, error) {
	resp := User{}
	r := dbs.Db.QueryRow(getUserStmt)
	if err := r.Scan(&resp.Name, &resp.Bio, &resp.Pic, &resp.DriveFolderId, &resp.DriveFolderName); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (dbs *DbService) UpdatePhoto(title string, description string, keywords []string, albums []string, driveId string) (*Photo, error) {
	if _, err := dbs.Db.Exec(updatePhotoStmt, title, description,
		trimAndJoin(keywords), trimAndJoin(albums), driveId); err != nil {
		return nil, err
	}
	return dbs.GetId(driveId, true)
}

func (dbs *DbService) UpdatePhotoDescription(description string, driveId string) (*Photo, error) {
	if _, err := dbs.Db.Exec(updatePhotoDescriptionStmt, description, driveId); err != nil {
		return nil, err
	}
	return dbs.GetId(driveId, true)
}

func (dbs *DbService) UpdatePhotoKeywords(keywords []string, driveId string) (*Photo, error) {
	if _, err := dbs.Db.Exec(updatePhotoKeywordsStmt, trimAndJoin(keywords), driveId); err != nil {
		return nil, err
	}
	return dbs.GetId(driveId, true)
}

func (dbs *DbService) UpdatePhotoTitle(title string, driveId string) (*Photo, error) {
	if _, err := dbs.Db.Exec(updatePhotoTitleStmt, title, driveId); err != nil {
		return nil, err
	}
	return dbs.GetId(driveId, true)
}

func (dbs *DbService) UpdatePhotoAlbum(albums []string, driveId string) (*Photo, error) {
	if _, err := dbs.Db.Exec(updatePhotoAlbumStmt, trimAndJoin(albums), driveId); err != nil {
		return nil, err
	}
	return dbs.GetId(driveId, true)
}

func (dbs *DbService) UpdatePhotoLikes(likes int, driveId string) (*Photo, error) {
	if _, err := dbs.Db.Exec(updatePhotoLikesStmt, likes, driveId); err != nil {
		return nil, err
	}
	return dbs.GetId(driveId, true)
}

func (dbs *DbService) UpdatePhotoPrivate(private bool, driveId string) (*Photo, error) {
	if _, err := dbs.Db.Exec(updatePhotoPrivateStmt, private, driveId); err != nil {
		return nil, err
	}
	return dbs.GetId(driveId, true)
}

func (dbs *DbService) UpdateUser(u *User) (*User, error) {
	if _, err := dbs.Db.Exec(updateUserStmt, u.Name, u.Bio, u.Pic); err != nil {
		return nil, err
	}
	return dbs.GetUser()
}

func (dbs *DbService) UpdateUserBio(bio string) (*User, error) {
	if _, err := dbs.Db.Exec(updateUserBioStmt, bio); err != nil {
		return nil, err
	}
	return dbs.GetUser()
}

func (dbs *DbService) UpdateUserName(name string) (*User, error) {
	if _, err := dbs.Db.Exec(updateUserNameStmt, name); err != nil {
		return nil, err
	}
	return dbs.GetUser()
}

func (dbs *DbService) UpdateUserPic(pic string) (*User, error) {
	if _, err := dbs.Db.Exec(updateUserPicStmt, pic); err != nil {
		return nil, err
	}
	return dbs.GetUser()
}

func (dbs *DbService) UpdateUserDriveFolder(id string, name string) (*User, error) {
	if _, err := dbs.Db.Exec(updateUserDriveFolder, id, name); err != nil {
		return nil, err
	}
	return dbs.GetUser()
}

func (dbs *DbService) GetAllPhotos(private bool) ([]*Photo, error) {
	var stmt = getAllPublic
	if private {
		stmt = getAll
	}
	if rows, err := dbs.Db.Query(stmt); err != nil {
		return nil, err
	} else {
		return scanR(rows)
	}
}

func (dbs *DbService) GetByOriginalDate(limit int, offset int, private bool) ([]*Photo, error) {
	var stmt = getByOriginalDatePublic
	if private {
		stmt = getByOriginalDate
	}
	if rows, err := dbs.Db.Query(stmt, limit, offset); err != nil {
		return nil, err
	} else {
		return scanR(rows)
	}
}

func (dbs *DbService) GetByDriveDate(limit int, offset int, private bool) ([]*Photo, error) {
	var stmt = getByDriveDatePublic
	if private {
		stmt = getByDriveDate
	}
	if rows, err := dbs.Db.Query(stmt, limit, offset); err != nil {
		return nil, err
	} else {
		return scanR(rows)
	}
}

func (dbs *DbService) GetByCameraModel(model string, private bool) ([]*Photo, error) {
	var stmt = getByCameraModelPublic
	if private {
		stmt = getByCameraModel
	}
	if rows, err := dbs.Db.Query(stmt, model); err != nil {
		return nil, err
	} else {
		return scanR(rows)
	}
}

func (dbs *DbService) GetLatest(private bool) (*Photo, error) {
	resp := Photo{}
	var stmt = getByDriveDatePublic
	if private {
		stmt = getByDriveDate
	}
	r := dbs.Db.QueryRow(stmt, 1, 0)
	if err := scanRow(&resp, r); err != nil {
		return nil, err
	} else {
		return &resp, nil
	}
}

func split(str string) []string {
	return strings.Split(str, ",")
}

func trimAndSplit(str string) []string {
	strs := split(str)
	var ret []string
	for _, s := range strs {
		ret = append(ret, strings.TrimSpace(s))
	}
	return ret
}

func trimAndJoin(strs []string) string {
	var newString []string
	for _, str := range strs {
		newString = append(newString, strings.TrimSpace(str))
	}
	return strings.Join(newString, ",")
}

func scanR(rows *sql.Rows) ([]*Photo, error) {
	var p *Photo
	var photos []*Photo
	for rows.Next() {
		p = &Photo{}
		if err := scanRows(p, rows); err == nil {
			photos = append(photos, p)
		} else {
			return nil, err
		}
	}
	return photos, nil
}

func scanRows(p *Photo, r *sql.Rows) error {
	err := r.Scan(&p.DriveId, &p.Md5, &p.FileName, &p.Title, &p.Keywords, &p.Description, &p.DriveDate,
		&p.OriginalDate, &p.CameraMake, &p.CameraModel, &p.LensMake, &p.LensModel, &p.FocalLength, &p.FocalLength35,
		&p.Iso, &p.FNumber, &p.Exposure, &p.Width, &p.Height, &p.Private, &p.Album, &p.Likes)

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
		&p.OriginalDate, &p.CameraMake, &p.CameraModel, &p.LensMake, &p.LensModel, &p.FocalLength, &p.FocalLength35,
		&p.Iso, &p.FNumber, &p.Exposure, &p.Width, &p.Height, &p.Private, &p.Album, &p.Likes)

	switch err {
	case sql.ErrNoRows:
		return NoSuchPhoto
	case nil:
		return nil
	default:
		return err
	}
}
