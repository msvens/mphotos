package dao

const schemaV1 = `
CREATE TABLE IF NOT EXISTS album (
	Id UUID,
	name TEXT,
	description TEXT NOT NULL,
	coverPic TEXT NOT NULL,
	CONSTRAINT album_name UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS albumphotos (
	albumId UUID,
	photoId UUID,
	PRIMARY KEY (albumId, photoId)
);

CREATE TABLE IF NOT EXISTS camera (
    id TEXT PRIMARY KEY,
    model TEXT NOT NULL,
	make TEXT NOT NULL,
	year INTEGER,
	effectivePixels INTEGER,
	totalPixels INTEGER,
	sensorSize TEXT,
	sensorType TEXT,
	sensorResolution TEXT,
	imageResolution TEXT,
	cropFactor REAL,
	opticalZoom REAL,
	digitalZoom BOOLEAN,
	iso TEXT,
	raw BOOLEAN,
	manualFocus BOOLEAN,
	focusRange INTEGER,
	macroFocusRange INTEGER,
	focalLengthEquiv TEXT,
	aperturePriority BOOLEAN,
	maxAperture TEXT,
	maxApertureEquiv TEXT,
	metering TEXT,
	exposureComp TEXT,
	shutterPriority BOOLEAN,
	minShutterSpeed TEXT,
	maxShutterSpeed TEXT,
	builtInFlash BOOLEAN,
	externalFlash BOOLEAN,
	viewFinder TEXT,
	videoCapture BOOLEAN,
	maxVideoResolution TEXT,
	gps BOOLEAN,
	image TEXT
);

CREATE INDEX IF NOT EXISTS model_idx ON camera (model);

CREATE TABLE IF NOT EXISTS comment (
	id SERIAL PRIMARY KEY,
	guestId UUID NOT NULL,
	photoId UUID NOT NULL,
	time TIMESTAMP NOT NULL,
	body TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS driveId_idx ON comment (photoId);

CREATE TABLE IF NOT EXISTS exifdata (
	id UUID PRIMARY KEY,
	data TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS guest (
	id UUID PRIMARY KEY,
	name TEXT NOT NULL,
	email TEXT NOT NULL,
	verified BOOLEAN NOT NULL,
	verifytime TIMESTAMP NOT NULL,
	CONSTRAINT guest_email UNIQUE (email),
	CONSTRAINT guest_name UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS reaction (
	guestId UUID,
	photoId UUID,
	kind TEXT,
	PRIMARY KEY (guestId, photoId)
);

CREATE TABLE IF NOT EXISTS photo (
    id UUID PRIMARY KEY,
	md5 TEXT NOT NULL,
	source TEXT NOT NULL,
	sourceId TEXT,
    sourceOther TEXT,
	sourceDate TIMESTAMP,
	uploadDate TIMESTAMP NOT NULL,
	originalDate TIMESTAMP NOT NULL,
	fileName TEXT NOT NULL,
	title TEXT NOT NULL,
	keywords TEXT,
	description TEXT,
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
	private BOOLEAN NOT NULL
);

CREATE TABLE IF NOT EXISTS usert (
	id INT PRIMARY KEY,
	name TEXT NOT NULL,
	bio TEXT NOT NULL,
	pic TEXT NOT NULL,
	driveFolderId TEXT NOT NULL,
	driveFolderName TEXT NOT NULL,
	config TEXT NOT NULL
);

INSERT INTO usert (id, name, bio, pic, driveFolderId, driveFolderName, config) VALUES (23657, '', '', '', '','','{}') ON CONFLICT (id) DO NOTHING;
`

const deleteSchemaV1 = `
DROP TABLE IF EXISTS album;
DROP TABLE IF EXISTS albumphotos;
DROP TABLE IF EXISTS camera;
DROP TABLE IF EXISTS comment;
DROP TABLE IF EXISTS exifdata;
DROP TABLE IF EXISTS guest;
DROP TABLE IF EXISTS reaction;
DROP TABLE IF EXISTS photo;
DROP TABLE IF EXISTS usert;
`