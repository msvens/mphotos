package model

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"
)

type CameraStore interface {
	AddCamera(camera *Camera) error
	AddCameraFromPhoto(photo *Photo) error
	CreateCameraStore() error
	Camera(id string) (*Camera, error)
	Cameras() ([]*Camera, error)
	DeleteCamera(id string) (bool, error)
	DeleteCameraStore() error
	HasCamera(id string) bool
	HasCameraModel(model string) bool
	PopulateFromPhotos() error
	UpdateCamera(camera *Camera) (*Camera, error)
	UpdateCameraImage(img, id string) (*Camera, error)
}

type Camera struct {
	Id                 string  `json:"id"`
	Model              string  `json:"model"`
	Make               string  `json:"make"`
	Year               int     `json:"year"`
	EffectivePixels    int     `json:"effectivePixels"`
	TotalPixels        int     `json:"totalPixels"`
	SensorSize         string  `json:"sensorSize"`
	SensorType         string  `json:"sensorType"`
	SensorResolution   string  `json:"sensorResolution"`
	ImageResolution    string  `json:"imageResolution"`
	CropFactor         float32 `json:"cropFactor"`
	OpticalZoom        float32 `json:"opticalZoom"`
	DigitalZoom        bool    `json:"digitalZoom"`
	Iso                string  `json:"iso"`
	Raw                bool    `json:"raw"`
	ManualFocus        bool    `json:"manualFocus"`
	FocusRange         int     `json:"focusRange"`
	MacroFocusRange    int     `json:"macroFocusRange"`
	FocalLengthEquiv   string  `json:"focalLengthEquiv"`
	AperturePriority   bool    `json:"aperturePriority"`
	MaxAperture        string  `json:"maxAperture"`
	MaxApertureEquiv   string  `json:"maxApertureEquiv"`
	Metering           string  `json:"metering"`
	ExposureComp       string  `json:"exposureComp"`
	ShutterPriority    bool    `json:"shutterPriority"`
	MinShutterSpeed    string  `json:"minShutterSpeed"`
	MaxShutterSpeed    string  `json:"maxShutterSpeed"`
	BuiltInFlash       bool    `json:"builtInFlash"`
	ExternalFlash      bool    `json:"externalFlash"`
	ViewFinder         string  `json:"viewFinder"`
	VideoCapture       bool    `json:"videoCapture"`
	MaxVideoResolution string  `json:"maxVideoResolution"`
	Gps                bool    `json:"gps"`
	Image              string  `json:"image"`
}

const cameraCols = "id,model,make,year,effectivePixels,totalPixels,sensorSize,sensorType,sensorResolution," +
	"imageResolution,cropFactor,opticalZoom,digitalZoom,iso,raw,manualFocus,focusRange,macroFocusRange," +
	"focalLengthEquiv,aperturePriority,maxAperture,maxApertureEquiv,metering,exposureComp,shutterPriority," +
	"minShutterSpeed,maxShutterSpeed,builtInFlash,externalFlash,viewFinder,videoCapture,maxVideoResolution," +
	"gps,image"

var NoSuchCamera = errors.New("No such Photo Id")
var space = regexp.MustCompile(`\s+`)

func convertModel(model string) string {
	return space.ReplaceAllString(strings.ToLower(model), "-")
}

func (db *DB) AddCamera(c *Camera) error {
	const insCamera = "INSERT INTO cameras (" + cameraCols + ") VALUES " +
		"($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23," +
		"$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34);"

	_, err := db.Exec(insCamera, convertModel(c.Model), c.Model, c.Make, c.Year, c.EffectivePixels, c.TotalPixels, c.SensorSize,
		c.SensorType, c.SensorResolution, c.ImageResolution, c.CropFactor, c.OpticalZoom, c.DigitalZoom,
		c.Iso, c.Raw, c.ManualFocus, c.FocusRange, c.MacroFocusRange, c.FocalLengthEquiv, c.AperturePriority,
		c.MaxAperture, c.MaxApertureEquiv, c.Metering, c.ExposureComp, c.ShutterPriority, c.MinShutterSpeed,
		c.MaxShutterSpeed, c.BuiltInFlash, c.ExternalFlash, c.ViewFinder, c.VideoCapture, c.MaxVideoResolution, c.Gps,
		c.Image)
	return err
}

func (db *DB) AddCameraFromPhoto(p *Photo) error {
	return db.AddCamera(&Camera{Model: p.CameraModel, Make: p.CameraMake})
}

func (db *DB) Camera(id string) (*Camera, error) {
	stmt := "SELECT " + cameraCols + " FROM cameras WHERE id = $1"
	if rows, err := db.Query(stmt, id); err != nil {
		return nil, err
	} else {
		defer rows.Close()
		return scanNextCamera(rows)
	}
}

func (db *DB) Cameras() ([]*Camera, error) {
	stmt := "SELECT " + cameraCols + " FROM cameras ORDER BY id"
	if rows, err := db.Query(stmt); err != nil {
		return nil, err
	} else {
		defer rows.Close()
		return scanCameras(rows)
	}
}

func (db *DB) CreateCameraStore() error {
	const stmt = `
CREATE TABLE IF NOT EXISTS cameras (
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
CREATE INDEX IF NOT EXISTS model_idx ON cameras (model);
`
	_, err := db.Exec(stmt)
	return err
}

func (db *DB) DeleteCamera(id string) (bool, error) {
	stmt := "DELETED FROM cameras WHERE id = $1"
	if res, err := db.Exec(stmt, id); err != nil {
		return false, err
	} else {
		cnt, _ := res.RowsAffected()
		return cnt > 0, nil
	}
}

func (db *DB) DeleteCameraStore() error {
	_, err := db.Exec("DROP TABLE IF EXISTS cameras;")
	return err
}

func (db *DB) HasCamera(id string) bool {
	stmt := "SELECT 1 FROM cameras WHERE id = $1"
	if rows, err := db.Query(stmt, id); err == nil {
		defer rows.Close()
		return rows.Next()
	} else {
		return false
	}
}

func (db *DB) HasCameraModel(model string) bool {
	return db.HasCamera(convertModel(model))
}

func (db *DB) PopulateFromPhotos() error {
	photos, err := db.Photos(Range{}, DriveDate, PhotoFilter{Private: true})
	if err != nil {
		return err
	}
	for _, p := range photos {
		if !db.HasCameraModel(p.CameraModel) {
			if err = db.AddCameraFromPhoto(p); err != nil {
				return err
			}
		}
	}
	return nil
}

func (db *DB) UpdateCamera(c *Camera) (*Camera, error) {
	const updateCols = "(make,year,effectivePixels,totalPixels,sensorSize,sensorType,sensorResolution," +
		"imageResolution,cropFactor,opticalZoom,digitalZoom,iso,raw,manualFocus,focusRange,macroFocusRange," +
		"focalLengthEquiv,aperturePriority,maxAperture,maxApertureEquiv,metering,exposureComp,shutterPriority," +
		"minShutterSpeed,maxShutterSpeed,builtInFlash,externalFlash,viewFinder,videoCapture,maxVideoResolution," +
		"gps) = ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24," +
		"$25,$26,$27,$28,$29,$30,$31)"

	const stmt = "UPDATE cameras SET " + updateCols + " WHERE id = $32"

	_, err := db.Exec(stmt, c.Make, c.Year, c.EffectivePixels, c.TotalPixels, c.SensorSize,
		c.SensorType, c.SensorResolution, c.ImageResolution, c.CropFactor, c.OpticalZoom, c.DigitalZoom,
		c.Iso, c.Raw, c.ManualFocus, c.FocusRange, c.MacroFocusRange, c.FocalLengthEquiv, c.AperturePriority,
		c.MaxAperture, c.MaxApertureEquiv, c.Metering, c.ExposureComp, c.ShutterPriority, c.MinShutterSpeed,
		c.MaxShutterSpeed, c.BuiltInFlash, c.ExternalFlash, c.ViewFinder, c.VideoCapture, c.MaxVideoResolution, c.Gps, c.Id)
	if err != nil {
		return nil, err
	}

	return db.Camera(c.Id)
}

func (db *DB) UpdateCameraImage(image, id string) (*Camera, error) {
	const stmt = "UPDATE cameras SET image = $1 WHERE id = $2"
	if _, err := db.Exec(stmt, image, id); err != nil {
		return nil, err
	}
	return db.Camera(id)
}

func scanCameras(rows *sql.Rows) ([]*Camera, error) {
	var c *Camera
	cameras := []*Camera{}
	for rows.Next() {
		c = &Camera{}
		if err := scanCamera(c, rows); err == nil {
			cameras = append(cameras, c)
		} else {
			return nil, err
		}
	}
	return cameras, nil
}

func scanNextCamera(rows *sql.Rows) (*Camera, error) {
	if rows.Next() {
		c := &Camera{}
		if err := scanCamera(c, rows); err != nil {
			return nil, err
		}
		return c, nil

	} else {
		return nil, NoSuchCamera
	}
}

func scanCamera(c *Camera, r *sql.Rows) error {
	err := r.Scan(&c.Id, &c.Model, &c.Make, &c.Year, &c.EffectivePixels, &c.TotalPixels, &c.SensorSize,
		&c.SensorType, &c.SensorResolution, &c.ImageResolution, &c.CropFactor, &c.OpticalZoom, &c.DigitalZoom,
		&c.Iso, &c.Raw, &c.ManualFocus, &c.FocusRange, &c.MacroFocusRange, &c.FocalLengthEquiv, &c.AperturePriority,
		&c.MaxAperture, &c.MaxApertureEquiv, &c.Metering, &c.ExposureComp, &c.ShutterPriority, &c.MinShutterSpeed,
		&c.MaxShutterSpeed, &c.BuiltInFlash, &c.ExternalFlash, &c.ViewFinder, &c.VideoCapture, &c.MaxVideoResolution, &c.Gps,
		&c.Image)

	switch err {
	case sql.ErrNoRows:
		return NoSuchCamera
	case nil:
		return nil
	default:
		return err
	}
}
