package dao

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/msvens/mexif"
	"time"
)

func MigrateFromModelToDAO() error {
	pgdb, err := NewPGDB()
	if err != nil {
		return err
	}

	err = pgdb.DeleteTables()
	if err != nil {
		return err
	}
	err = pgdb.CreateTables()

	/*
		mdb, err := model.NewDB()
		if err != nil {
			return err
		}
	*/
	//migrate entities
	if err = migrateCamera(pgdb); err != nil {
		fmt.Println("migrating camera error")
		return err
	}

	if err = migrateGuests(pgdb); err != nil {
		fmt.Println("migrating guests error")
		return err
	}

	if err = migratePhotos(pgdb); err != nil {
		fmt.Println("migrating photos error")
		return err
	}

	if err = migrateUsers(pgdb); err != nil {
		return err
	}

	if err = migrateComments(pgdb); err != nil {
		return err
	}

	if err = migrateLikes(pgdb); err != nil {
		return err
	}

	if err = migrateAlbums(pgdb); err != nil {
		return nil
	}

	return nil
}

type OldAlbumPhoto struct {
	AlbumId int
	DriveId string
}

type OldAlbum struct {
	Id          int
	Name        string
	Description string
	CoverPic    string
}

func migrateAlbums(pgdb *PGDB) error {
	oldAlbums := []OldAlbum{}
	if err := pgdb.db.Select(&oldAlbums, "SELECT * from Albums"); err != nil {
		return err
	}
	if len(oldAlbums) < 1 {
		return nil
	}
	oldAlbumPhotos := []OldAlbumPhoto{}
	if err := pgdb.db.Select(&oldAlbumPhotos, "SELECT * FROM albumphoto"); err != nil {
		return err
	}

	//DriveId <-> PhotoMapping
	drivePhotos, err := pgdb.Photo.ListSource(SourceGoogle)
	if err != nil {
		return err
	}
	pMap := make(map[string]uuid.UUID)
	for _, p := range drivePhotos {
		pMap[p.SourceId] = p.Id
	}
	//Insert album and add id <-> uuid mapping
	aMap := make(map[int]uuid.UUID)
	for _, a := range oldAlbums {
		if aNew, err := pgdb.Album.Add(a.Name, a.Description, a.CoverPic); err != nil {
			return err
		} else {
			aMap[a.Id] = aNew.Id
		}
	}
	//Insert albumPhoto
	stmt := "INSERT INTO albumphotos (albumId, photoId) VALUES ($1, $2)"
	for _, ap := range oldAlbumPhotos {
		photoId, found := pMap[ap.DriveId]
		if !found {
			return fmt.Errorf("Could not find photo Id for Drive Id")
		}
		albumId, found := aMap[ap.AlbumId]
		if !found {
			return fmt.Errorf("Could not find AlbumId for old Album Id")
		}
		if _, err := pgdb.db.Exec(stmt, albumId, photoId); err != nil {
			return err
		}
	}
	return nil

}

type OldLike struct {
	Guest   uuid.UUID
	DriveId string
}

func migrateLikes(pgdb *PGDB) error {
	oldLikes := []OldLike{}

	err := pgdb.db.Select(&oldLikes, "SELECT * FROM likes")
	if err != nil {
		return err
	}

	drivePhotos, err := pgdb.Photo.ListSource(SourceGoogle)
	if err != nil {
		return err
	}
	pMap := make(map[string]uuid.UUID)
	for _, p := range drivePhotos {
		pMap[p.SourceId] = p.Id
	}
	for _, l := range oldLikes {
		if pId, found := pMap[l.DriveId]; !found {
			println("skipping like could not find photo")
		} else if !pgdb.Guest.Has(l.Guest) {
			println("skipping like...could not find user")
		} else {
			r := Reaction{l.Guest, pId, "Like"}
			err = pgdb.Reaction.Add(&r)
			if err != nil {
				return err
			}
		}

	}
	return nil

}

func migrateUsers(pgdb *PGDB) error {
	olduser := User{}
	if err := pgdb.db.Get(&olduser, "SELECT name, bio, pic, driveFolderId, driveFolderName, config FROM USERS LIMIT 1"); err != nil {
		return err
	}
	u := User{}
	u.Bio = olduser.Bio
	u.Pic = olduser.Pic
	u.Name = olduser.Name
	u.DriveFolderName = olduser.DriveFolderName
	u.DriveFolderId = olduser.DriveFolderId
	u.Config = olduser.Config
	_, err := pgdb.User.Update(&u)

	return err
}

func migrateCamera(pgdb *PGDB) error {
	oldcameras := []Camera{}
	if err := pgdb.db.Select(&oldcameras, "SELECT * FROM CAMERAS"); err != nil {
		return nil
	}
	for _, c := range oldcameras {
		if err := pgdb.Camera.Add(&c); err != nil {
			println("could not add new camera")
			return err
		}
	}
	return nil
}

type OldComment struct {
	Id      int
	Guestid uuid.UUID
	Driveid string
	Ts      time.Time
	Body    string
}

func migrateComments(pgdb *PGDB) error {
	//get old comments

	oldcomments := []OldComment{}

	err := pgdb.db.Select(&oldcomments, "SELECT * from comments")
	if err != nil {
		return err
	}
	drivePhotos, err := pgdb.Photo.ListSource(SourceGoogle)
	if err != nil {
		return err
	}
	pMap := make(map[string]uuid.UUID)
	for _, p := range drivePhotos {
		pMap[p.SourceId] = p.Id
	}
	stmt := buildInsertNamed("comment", getStructFields(Comment{}), "id")
	for _, c := range oldcomments {
		if !pgdb.Guest.Has(c.Guestid) {
			println("skipping comment with no user")
		} else {
			if pId, found := pMap[c.Driveid]; !found {
				println("skpping comment with non existent photo")
			} else {
				newC := Comment{GuestId: c.Guestid, PhotoId: pId, Body: c.Body, Time: c.Ts}
				if _, err := pgdb.db.NamedExec(stmt, &newC); err != nil {
					return err
				}
			}
		}
	}
	/*for _, oldcomment := range oldcomments {
		pgdb.Comment.Add()
	}*/
	return nil
}

func migrateGuests(pgdb *PGDB) error {
	oldguests := []Guest{}
	err := pgdb.db.Select(&oldguests, "SELECT * from guests")
	if err != nil {
		return err
	}

	fields := getStructFields(Guest{})
	stmt := buildInsertNamed("guest", fields)
	for _, g := range oldguests {
		if _, err = pgdb.db.NamedExec(stmt, &g); err != nil {
			return err
		}
	}
	return nil
}

type OldPhoto struct {
	DriveId      string
	Md5          string
	FileName     string
	Title        string
	Keywords     string
	Description  string
	DriveDate    time.Time
	OriginalDate time.Time

	CameraMake    string
	CameraModel   string
	LensMake      string
	LensModel     string
	FocalLength   string
	FocalLength35 string

	Iso      uint
	Exposure string
	FNumber  float32
	Width    uint
	Height   uint
	Private  bool
	Likes    uint
}

func migratePhotos(pgdb *PGDB) error {
	var err error
	//Get all photos
	oldphotos := []OldPhoto{}
	if err := pgdb.db.Select(&oldphotos, "SELECT * FROM photos"); err != nil {
		return nil
	}
	//oldphotos, err := mdb.Photos(model.Range{}, model.DriveDate, model.PhotoFilter{true, ""})
	if err != nil {
		return err
	}
	for _, p := range oldphotos {
		nid := uuid.New()
		newPhoto := Photo{}
		newPhoto.Id = nid
		newPhoto.Md5 = p.Md5
		newPhoto.Source = SourceGoogle
		newPhoto.SourceId = p.DriveId
		newPhoto.SourceOther = ""
		newPhoto.SourceDate = p.DriveDate
		newPhoto.UploadDate = p.DriveDate
		newPhoto.OriginalDate = p.OriginalDate
		newPhoto.FileName = p.FileName
		newPhoto.Keywords = p.Keywords
		newPhoto.Description = p.Description
		newPhoto.CameraMake = p.CameraMake
		newPhoto.CameraModel = p.CameraModel
		newPhoto.LensMake = p.LensMake
		newPhoto.LensModel = p.LensModel
		newPhoto.FocalLength = p.FocalLength
		newPhoto.FocalLength35 = p.FocalLength35
		newPhoto.Iso = p.Iso
		newPhoto.Exposure = p.Exposure
		newPhoto.FNumber = p.FNumber
		newPhoto.Width = p.Width
		newPhoto.Height = p.Height
		newPhoto.Private = p.Private

		//get exif information
		exifdata := ""
		if e := pgdb.db.Get(&exifdata, "SELECT data FROM exif WHERE driveid = $1", p.DriveId); e != nil {
			return nil
		}
		resp := Exif{Data: &mexif.ExifCompact{}}
		if err := json.Unmarshal([]byte(exifdata), resp.Data); err != nil {
			return err
		}
		err = pgdb.Photo.Add(&newPhoto, resp.Data)
		if err != nil {
			return err
		}
	}
	return err
}