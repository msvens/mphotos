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

type Comments struct {
	DriveId  string   `json:"driveId"`
	Comments []string `json:"comments"`
}

type Album struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CoverPic    string `json:"coverPic"`
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

type UXConfig struct {
}

const createAlbumTable = `
	CREATE TABLE IF NOT EXISTS albums (
		name TEXT PRIMARY KEY,
		description TEXT NOT NULL,
		coverPic TEXT NOT NULL
	);
`
const createAlbumPhotoTable = `
	CREATE TABLE IF NOT EXISTS albumphoto (
		album TEXT,
		driveId TEXT,
		PRIMARY KEY (album, driveId)
	);
`

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
	height INTEGER NOT NULL,
	private BOOLEAN NOT NULL,
	likes INTEGER NOT NULL
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

const dropAlbumTable = "DROP TABLE IF EXISTS albums;"

const dropAlbumPhotoTable = "DROP TABLE IF EXISTS albumphoto;"

const allCols = "driveId,md5,fileName,title,keywords,description,driveDate,originalDate," +
	"cameraMake,cameraModel,lensMake,lensModel,focalLength,focalLength35,iso,fNumber,exposure," +
	"width,height,private,likes"

const (
	containsIdStmt           = "SELECT 1 FROM photos WHERE driveId = $1"
	containsIdPublicStmt     = "SELECT 1 FROM photos WHERE driveId = $1 AND private = false"
	containsAlbumStmt        = "SELECT 1 FROM albums WHERE name = $1"
	containsAlbumPhotoStmt   = "SELECT 1 FROM albumphoto WHERE name = $1 AND driveId = $2"
	deleteStmt               = "DELETE FROM photos WHERE driveId = $1;"
	deleteExifStmt           = "DELETE FROM exif WHERE driveId = $1"
	distinctAlbumsStmt       = "SELECT DISTINCT(album) FROM photos"
	getExifStmt              = "SELECT " + allCols + " FROM exif WHERE driveId = $1"
	getIdStmt                = "SELECT " + allCols + " FROM photos WHERE driveId = $1"
	getIdStmtPublic          = "SELECT " + allCols + " FROM photos WHERE driveId = $1 AND private = false"
	getAlbumsStmt            = "SELECT name,description,coverPic FROM albums"
	getAlbumStmt             = "SELECT name,description,coverPic FROM albums WHERE name = $1"
	getAlbumPhotosPublicStmt = "SELECT " + allCols + ` FROM photos WHERE private = false AND 
									driveId IN (SELECT driveId FROM albumphoto WHERE album = $1)`
	getAlbumPhotosStmt         = "SELECT " + allCols + " FROM photos WHERE driveId IN (SELECT driveId FROM albumphoto WHERE album = $1)"
	getAll                     = "SELECT " + allCols + " FROM photos"
	getByDriveDate             = "SELECT " + allCols + " FROM photos ORDER BY driveDate DESC LIMIT $1 OFFSET $2"
	getByOriginalDate          = "SELECT " + allCols + " FROM photos ORDER BY originalDate DESC LIMIT $1 OFFSET $2"
	getByCameraModel           = "SELECT " + allCols + " FROM photos WHERE cameraModel = $1 ORDER BY driveDate DESC"
	getAllPublic               = "SELECT " + allCols + " FROM photos WHERE private = false"
	getByDriveDatePublic       = "SELECT " + allCols + " FROM photos WHERE private = false ORDER BY driveDate DESC LIMIT $1 OFFSET $2"
	getByOriginalDatePublic    = "SELECT " + allCols + " FROM photos WHERE private = false ORDER BY originalDate DESC LIMIT $1 OFFSET $2"
	getByCameraModelPublic     = "SELECT " + allCols + " FROM photos WHERE private = false AND cameraModel = $1 ORDER BY driveDate DESC"
	getPhotoAlbumsStmt         = "SELECT album FROM albumphoto WHERE driveId = $1"
	getUserStmt                = "SELECT name,bio,pic,driveFolderId,driveFolderName FROM users LIMIT 1"
	updateAlbumStmt            = "UPDATE albums SET (description, coverPic) = ($1, $2) WHERE name = $3"
	updatePhotoTitleStmt       = "UPDATE photos SET title = $1 WHERE driveId = $2"
	updatePhotoDescriptionStmt = "UPDATE photos SET description = $1 WHERE driveId = $2"
	updatePhotoKeywordsStmt    = "UPDATE photos SET keywords = $1 WHERE driveId = $2"
	//updatePhotoAlbumStmt       = "UPDATE photos SET album = $1 WHERE driveId = $2"
	updatePhotoPrivateStmt = "UPDATE photos SET private = $1 WHERE driveId = $2"
	updatePhotoLikesStmt   = "UPDATE photos SET likes = $1 WHERE driveId = $2"
	updatePhotoStmt        = "UPDATE photos SET (title, description,keywords) = ($1, $2, $3) WHERE driveId = $4"
	updateUserBioStmt      = "UPDATE users SET bio = $1"
	updateUserNameStmt     = "UPDATE users SET name = $1"
	updateUserPicStmt      = "UPDATE users SET pic = $1"
	updateUserDriveFolder  = "UPDATE users SET (driveFolderId, driveFolderName) = ($1, $2)"
	updateUserStmt         = "UPDATE users SET (name, bio, pic) = ($1, $2, $3)"
	insAlbumStmt           = "INSERT INTO albums (name, description, coverPic) VALUES ($1, $2, $3)"
	insAlbumPhotoStmt      = "INSERT INTO albumphoto (album, driveId) VALUES ($1, $2)"
	insExifStmt            = "INSERT INTO exif (driveId, data) VALUES ($1, $2)"
	insPhotoStmt           = `
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
	height,
	private,
	likes) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21);
`
	insUserStmt = "INSERT INTO user (name, bio, pic, driveFolderId) VALUES ($1, $2, $3, $4)"
)
