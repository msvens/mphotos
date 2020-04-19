package service

import (
	"github.com/msvens/mexif"
	"time"
)

type Photo struct {
	DriveId string `json:"driveId"`
	Md5 string `json:"md5"`
	FileName string `json:"fileName"`
	Title string `json:"title"`
	Keywords string `json:"keywords,omitempty"`
	Description string `json:"description,omitempty"`
	DriveDate time.Time `json:"driveDate"`
	OriginalDate time.Time `json:"originalDate"`

	CameraMake string `json:"cameraMake"`
	CameraModel string `json:"cameraModel"`
	LensMake string `json:"lensMake,omitempty"`
	LensModel string `json:"lensModel,omitempty"`

	Iso uint `json:"iso"`
	Exposure string `json:"exposure"`
	FNumber float32 `json:"fNumber"`

}

type Exif struct {
	DriveId string
	Data *mexif.ExifCompact
}

type User struct {
	Name string `json:"name"`
	Bio string `json:"bio"`
	Pic string `json:pic`
	DriveFolderId string `json:driveFolderId`
}

const createPhotoTable = `
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
 	iso INTEGER NOT NULL,
	fNumber REAL NOT NULL,
	exposure TEXT NOT NULL
);
`

const createExifTable = `
CREATE TABLE IF NOT EXISTS exif (
	driveId TEXT PRIMARY KEY,
	data TEXT NOT NULL
);
`

const createUserTable = `
CREATE TABLE IF NOT EXISTS user (
	id SERIAL PRIMARY KEY,
	name TEXT,
	bio TEXT,
	pic TEXT,
	driveFolderId TEXT
);
`

const dropPhotoTable = "DROP TABLE IF EXISTS photos"

const dropExifTable = "DROP TABLE IF EXISTS exif;"

const dropUserTable = "DROP TABLE IF EXISTS users;"

const (
	containsUserStmt = "SELECT 1 FROM user"
	containsIdStmt = "SELECT 1 FROM photos WHERE driveId = $1"
	getExifStmt = "SELECT * FROM exif WHERE driveId = $1"
	getIdStmt = "SELECT * FROM photos WHERE driveid = $1"
	getStmt = "SELECT * FROM photos"
	getUserStmt = "SELECT name,bio,pic,driveFolderId FROM user LIMIT 1"
	insExifStmt = "INSERT INTO exif (driveId, data) VALUES ($1, $2)"
	insPhotoStmt = `
INSERT INTO photos (
	driveId,
	md5, 
	fileName, 
	title,
	keywords,
	description, 
	driveDate, 
	originalDate,
	cameraMake,
	cameraModel,
	lensMake,
	lensModel,
	iso,
	fNumber,
	exposure) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14, $15);
`
	insUserStmt = "INSERT INTO user (name, bio, pic, driveFolderId) VALUES ($1, $2, $3, $4)"
)


