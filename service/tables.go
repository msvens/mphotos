package service

import (
	"github.com/msvens/mexif"
	"time"
)

type Photo struct {
	DriveId      string    `json:"driveId"`
	Md5          string    `json:"md5"`
	FileName     string    `json:"fileName"`
	Title        string    `json:"title"`
	Keywords     string    `json:"keywords,omitempty"`
	Description  string    `json:"description,omitempty"`
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
}

type Exif struct {
	DriveId string
	Data    *mexif.ExifCompact
}

type User struct {
	Name            string `json:"name"`
	Bio             string `json:"bio"`
	Pic             string `json:"pic"`
	DriveFolderId   string `json:"driveFolderId,omitempty"`
	DriveFolderName string `json:"driveFolderName,omitempty"`
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
	focalLength TEXT,
	focalLength35 TEXT,
 	iso INTEGER NOT NULL,
	fNumber REAL NOT NULL,
	exposure TEXT NOT NULL,
	width INTEGER NOT NULL,
	height INTEGER NOT NULL
);
`

const createExifTable = `
CREATE TABLE IF NOT EXISTS exif (
	driveId TEXT PRIMARY KEY,
	data TEXT NOT NULL
);
`
const UserId = 23657

const createUserTable = `
CREATE TABLE IF NOT EXISTS users (
	id INT PRIMARY KEY,
	name TEXT NOT NULL,
	bio TEXT NOT NULL,
	pic TEXT NOT NULL,
	driveFolderId TEXT NOT NULL,
	driveFolderName TEXT NOT NULL
);
INSERT INTO users (id, name, bio, pic, driveFolderId, driveFolderName) VALUES (23657, '', '', '', '','') ON CONFLICT (id) DO NOTHING;
`

const dropPhotoTable = "DROP TABLE IF EXISTS photos;"

const dropExifTable = "DROP TABLE IF EXISTS exif;"

const dropUserTable = "DROP TABLE IF EXISTS users;"

const (
	containsIdStmt        = "SELECT 1 FROM photos WHERE driveId = $1"
	deleteStmt            = "DELETE FROM photos WHERE driveId = $1;"
	deleteExifStmt        = "DELETE FROM exif WHERE driveId = $1"
	getExifStmt           = "SELECT * FROM exif WHERE driveId = $1"
	getIdStmt             = "SELECT * FROM photos WHERE driveid = $1"
	getAll                = "SELECT * FROM photos"
	getByDriveDate        = "SELECT * FROM photos ORDER BY drivedate DESC LIMIT $1 OFFSET $2"
	getByOriginalDate     = "SELECT * FROM photos ORDER BY originaldate DESC LIMIT $1 OFFSET $2"
	getUserStmt           = "SELECT name,bio,pic,driveFolderId,driveFolderName FROM users LIMIT 1"
	updateUserBioStmt     = "UPDATE users SET bio = $1"
	updateUserNameStmt    = "UPDATE users SET name = $1"
	updateUserPicStmt     = "UPDATE users SET pic = $1"
	updateUserDriveFolder = "UPDATE users SET (driveFolderId, driveFolderName) = ($1, $2)"
	updateUserStmt        = "UPDATE users SET (name, bio, pic) = ($1, $2, $3)"
	insExifStmt           = "INSERT INTO exif (driveId, data) VALUES ($1, $2)"
	insPhotoStmt          = `
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
	focalLength,
    focalLength35,
	iso,
	fNumber,
	exposure,
	width,
	height) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19);
`
	insUserStmt = "INSERT INTO user (name, bio, pic, driveFolderId) VALUES ($1, $2, $3, $4)"
)
