package dao

// schemaV4toV5 enriches the guest profile and adds the one-time code store.
// upgradeToV5 also grandfathers existing guests to verified=true (see migrate.go).
const schemaV4toV5 = `
	ALTER TABLE guest ADD COLUMN fullname TEXT NOT NULL DEFAULT '';
	ALTER TABLE guest ADD COLUMN description TEXT NOT NULL DEFAULT '';
	CREATE TABLE IF NOT EXISTS guestcode (
		guestid UUID NOT NULL,
		purpose TEXT NOT NULL,
		code    TEXT NOT NULL,
		expires TIMESTAMP NOT NULL,
		PRIMARY KEY (guestid, purpose)
	);
`

// schemaV5toV6 adds the guest avatar reference column (the file extension of the
// uploaded avatar, e.g. ".jpg"; empty means no avatar).
const schemaV5toV6 = `
	ALTER TABLE guest ADD COLUMN avatar TEXT NOT NULL DEFAULT '';
`

// schemaV6toV7 adds the Google account id (OIDC 'sub') for guests who sign in
// with Google. The partial unique index keeps it unique among linked guests
// while allowing many ” (email-only) guests.
const schemaV6toV7 = `
	ALTER TABLE guest ADD COLUMN googleid TEXT NOT NULL DEFAULT '';
	CREATE UNIQUE INDEX guest_googleid_idx ON guest (googleid) WHERE googleid <> '';
`

// schemaV7toV8 normalizes photos/cameras that were imported without camera EXIF
// to the "No Camera" sentinel, replacing the blank camera row those produced.
// Only the camera model is touched; no other photo field changes. Idempotent.
const schemaV7toV8 = `
	UPDATE img SET cameramodel = 'No Camera' WHERE cameramodel = '';
	DELETE FROM camera WHERE id = '';
	INSERT INTO camera (id, model, make) VALUES ('no-camera', 'No Camera', '') ON CONFLICT (id) DO NOTHING;
`

// schemaV8toV9 makes every nullable camera column non-null. A single NULL in any
// of them aborted the row scan for the whole /api/cameras list, because the Go
// Camera struct scans them into non-pointer int/float/bool/string. Backfill
// existing NULLs to the type's zero value, then pin NOT NULL DEFAULT so new rows
// can't reintroduce them (e.g. the v8 'no-camera' sentinel, which was inserted
// with only id/model/make and left the rest NULL). Idempotent.
const schemaV8toV9 = `
UPDATE camera SET
	year = COALESCE(year, 0),
	effectivePixels = COALESCE(effectivePixels, 0),
	totalPixels = COALESCE(totalPixels, 0),
	sensorSize = COALESCE(sensorSize, ''),
	sensorType = COALESCE(sensorType, ''),
	sensorResolution = COALESCE(sensorResolution, ''),
	imageResolution = COALESCE(imageResolution, ''),
	cropFactor = COALESCE(cropFactor, 0),
	opticalZoom = COALESCE(opticalZoom, 0),
	digitalZoom = COALESCE(digitalZoom, FALSE),
	iso = COALESCE(iso, ''),
	raw = COALESCE(raw, FALSE),
	manualFocus = COALESCE(manualFocus, FALSE),
	focusRange = COALESCE(focusRange, 0),
	macroFocusRange = COALESCE(macroFocusRange, 0),
	focalLengthEquiv = COALESCE(focalLengthEquiv, ''),
	aperturePriority = COALESCE(aperturePriority, FALSE),
	maxAperture = COALESCE(maxAperture, ''),
	maxApertureEquiv = COALESCE(maxApertureEquiv, ''),
	metering = COALESCE(metering, ''),
	exposureComp = COALESCE(exposureComp, ''),
	shutterPriority = COALESCE(shutterPriority, FALSE),
	minShutterSpeed = COALESCE(minShutterSpeed, ''),
	maxShutterSpeed = COALESCE(maxShutterSpeed, ''),
	builtInFlash = COALESCE(builtInFlash, FALSE),
	externalFlash = COALESCE(externalFlash, FALSE),
	viewFinder = COALESCE(viewFinder, ''),
	videoCapture = COALESCE(videoCapture, FALSE),
	maxVideoResolution = COALESCE(maxVideoResolution, ''),
	gps = COALESCE(gps, FALSE),
	image = COALESCE(image, '');

ALTER TABLE camera
	ALTER COLUMN year SET DEFAULT 0, ALTER COLUMN year SET NOT NULL,
	ALTER COLUMN effectivePixels SET DEFAULT 0, ALTER COLUMN effectivePixels SET NOT NULL,
	ALTER COLUMN totalPixels SET DEFAULT 0, ALTER COLUMN totalPixels SET NOT NULL,
	ALTER COLUMN sensorSize SET DEFAULT '', ALTER COLUMN sensorSize SET NOT NULL,
	ALTER COLUMN sensorType SET DEFAULT '', ALTER COLUMN sensorType SET NOT NULL,
	ALTER COLUMN sensorResolution SET DEFAULT '', ALTER COLUMN sensorResolution SET NOT NULL,
	ALTER COLUMN imageResolution SET DEFAULT '', ALTER COLUMN imageResolution SET NOT NULL,
	ALTER COLUMN cropFactor SET DEFAULT 0, ALTER COLUMN cropFactor SET NOT NULL,
	ALTER COLUMN opticalZoom SET DEFAULT 0, ALTER COLUMN opticalZoom SET NOT NULL,
	ALTER COLUMN digitalZoom SET DEFAULT FALSE, ALTER COLUMN digitalZoom SET NOT NULL,
	ALTER COLUMN iso SET DEFAULT '', ALTER COLUMN iso SET NOT NULL,
	ALTER COLUMN raw SET DEFAULT FALSE, ALTER COLUMN raw SET NOT NULL,
	ALTER COLUMN manualFocus SET DEFAULT FALSE, ALTER COLUMN manualFocus SET NOT NULL,
	ALTER COLUMN focusRange SET DEFAULT 0, ALTER COLUMN focusRange SET NOT NULL,
	ALTER COLUMN macroFocusRange SET DEFAULT 0, ALTER COLUMN macroFocusRange SET NOT NULL,
	ALTER COLUMN focalLengthEquiv SET DEFAULT '', ALTER COLUMN focalLengthEquiv SET NOT NULL,
	ALTER COLUMN aperturePriority SET DEFAULT FALSE, ALTER COLUMN aperturePriority SET NOT NULL,
	ALTER COLUMN maxAperture SET DEFAULT '', ALTER COLUMN maxAperture SET NOT NULL,
	ALTER COLUMN maxApertureEquiv SET DEFAULT '', ALTER COLUMN maxApertureEquiv SET NOT NULL,
	ALTER COLUMN metering SET DEFAULT '', ALTER COLUMN metering SET NOT NULL,
	ALTER COLUMN exposureComp SET DEFAULT '', ALTER COLUMN exposureComp SET NOT NULL,
	ALTER COLUMN shutterPriority SET DEFAULT FALSE, ALTER COLUMN shutterPriority SET NOT NULL,
	ALTER COLUMN minShutterSpeed SET DEFAULT '', ALTER COLUMN minShutterSpeed SET NOT NULL,
	ALTER COLUMN maxShutterSpeed SET DEFAULT '', ALTER COLUMN maxShutterSpeed SET NOT NULL,
	ALTER COLUMN builtInFlash SET DEFAULT FALSE, ALTER COLUMN builtInFlash SET NOT NULL,
	ALTER COLUMN externalFlash SET DEFAULT FALSE, ALTER COLUMN externalFlash SET NOT NULL,
	ALTER COLUMN viewFinder SET DEFAULT '', ALTER COLUMN viewFinder SET NOT NULL,
	ALTER COLUMN videoCapture SET DEFAULT FALSE, ALTER COLUMN videoCapture SET NOT NULL,
	ALTER COLUMN maxVideoResolution SET DEFAULT '', ALTER COLUMN maxVideoResolution SET NOT NULL,
	ALTER COLUMN gps SET DEFAULT FALSE, ALTER COLUMN gps SET NOT NULL,
	ALTER COLUMN image SET DEFAULT '', ALTER COLUMN image SET NOT NULL;
`
const schemaV9 = `
CREATE TABLE IF NOT EXISTS album (
	Id UUID,
	name TEXT,
	description TEXT NOT NULL,
	coverPic TEXT NOT NULL,
	code TEXT NOT NULL,
	orderBy INTEGER NOT NULL,
	CONSTRAINT album_name UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS albumphotos (
	albumId UUID,
	photoId UUID,
	photoOrder INTEGER,
	PRIMARY KEY (albumId, photoId)
);

CREATE TABLE IF NOT EXISTS camera (
    id TEXT PRIMARY KEY,
    model TEXT NOT NULL,
	make TEXT NOT NULL,
	year INTEGER NOT NULL DEFAULT 0,
	effectivePixels INTEGER NOT NULL DEFAULT 0,
	totalPixels INTEGER NOT NULL DEFAULT 0,
	sensorSize TEXT NOT NULL DEFAULT '',
	sensorType TEXT NOT NULL DEFAULT '',
	sensorResolution TEXT NOT NULL DEFAULT '',
	imageResolution TEXT NOT NULL DEFAULT '',
	cropFactor REAL NOT NULL DEFAULT 0,
	opticalZoom REAL NOT NULL DEFAULT 0,
	digitalZoom BOOLEAN NOT NULL DEFAULT FALSE,
	iso TEXT NOT NULL DEFAULT '',
	raw BOOLEAN NOT NULL DEFAULT FALSE,
	manualFocus BOOLEAN NOT NULL DEFAULT FALSE,
	focusRange INTEGER NOT NULL DEFAULT 0,
	macroFocusRange INTEGER NOT NULL DEFAULT 0,
	focalLengthEquiv TEXT NOT NULL DEFAULT '',
	aperturePriority BOOLEAN NOT NULL DEFAULT FALSE,
	maxAperture TEXT NOT NULL DEFAULT '',
	maxApertureEquiv TEXT NOT NULL DEFAULT '',
	metering TEXT NOT NULL DEFAULT '',
	exposureComp TEXT NOT NULL DEFAULT '',
	shutterPriority BOOLEAN NOT NULL DEFAULT FALSE,
	minShutterSpeed TEXT NOT NULL DEFAULT '',
	maxShutterSpeed TEXT NOT NULL DEFAULT '',
	builtInFlash BOOLEAN NOT NULL DEFAULT FALSE,
	externalFlash BOOLEAN NOT NULL DEFAULT FALSE,
	viewFinder TEXT NOT NULL DEFAULT '',
	videoCapture BOOLEAN NOT NULL DEFAULT FALSE,
	maxVideoResolution TEXT NOT NULL DEFAULT '',
	gps BOOLEAN NOT NULL DEFAULT FALSE,
	image TEXT NOT NULL DEFAULT ''
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
	fullname TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	avatar TEXT NOT NULL DEFAULT '',
	googleid TEXT NOT NULL DEFAULT '',
	verified BOOLEAN NOT NULL,
	verifytime TIMESTAMP NOT NULL,
	CONSTRAINT guest_email UNIQUE (email),
	CONSTRAINT guest_name UNIQUE (name)
);

CREATE UNIQUE INDEX IF NOT EXISTS guest_googleid_idx ON guest (googleid) WHERE googleid <> '';

CREATE TABLE IF NOT EXISTS guestcode (
	guestid UUID NOT NULL,
	purpose TEXT NOT NULL,
	code    TEXT NOT NULL,
	expires TIMESTAMP NOT NULL,
	PRIMARY KEY (guestid, purpose)
);

CREATE TABLE IF NOT EXISTS reaction (
	guestId UUID,
	photoId UUID,
	kind TEXT,
	PRIMARY KEY (guestId, photoId)
);

CREATE TABLE IF NOT EXISTS img (
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
	height INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS usert (
	id INT PRIMARY KEY,
	name TEXT NOT NULL,
	bio TEXT NOT NULL,
	pic TEXT NOT NULL,
	driveFolderId TEXT NOT NULL,
	driveFolderName TEXT NOT NULL,
	photostreamalbumid TEXT NOT NULL DEFAULT '',
	config TEXT NOT NULL
);

CREATE TABLE version (
	id bool PRIMARY KEY DEFAULT TRUE,
	versionId INT NOT NULL,
    description TEXT,
    CONSTRAINT version_unique CHECK (id)
);


INSERT INTO version (versionId,description) VALUES (0,'no version set') ON CONFLICT (id) DO NOTHING;
INSERT INTO usert (id, name, bio, pic, driveFolderId, driveFolderName, config) VALUES (23657, '', '', '', '','','{}') ON CONFLICT (id) DO NOTHING;
`

const deleteSchemaV9 = `
DROP TABLE IF EXISTS album;
DROP TABLE IF EXISTS albumphotos;
DROP TABLE IF EXISTS camera;
DROP TABLE IF EXISTS comment;
DROP TABLE IF EXISTS exifdata;
DROP TABLE IF EXISTS guest;
DROP TABLE IF EXISTS guestcode;
DROP TABLE IF EXISTS reaction;
DROP TABLE IF EXISTS img;
DROP TABLE IF EXISTS usert;
DROP TABLE IF EXISTS version;
`
