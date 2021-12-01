package dao

import (
	"github.com/google/uuid"
	"github.com/msvens/mimage/metadata"
	"time"
)

type Album struct {
	Id          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CoverPic    string    `json:"coverPic"`
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

type Comment struct {
	Id      int       `json:"id"`
	GuestId uuid.UUID `json:"-"`
	PhotoId uuid.UUID `json:"photoId"`
	Time    time.Time `json:"time"`
	Body    string    `json:"body"`
}

type Exif struct {
	Id   uuid.UUID                 `json:"id"`
	Data *metadata.MetaDataSummary `json:"data"`
}

type Guest struct {
	Id         uuid.UUID `json:"-"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	Verified   bool      `json:"verified"`
	VerifyTime time.Time `json:"verifyTime"`
}

type GuestReaction struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
}

type Photo struct {
	Id           uuid.UUID `json:"id"`
	Md5          string    `json:"md5"`
	Source       string    `json:"source"`
	SourceId     string    `json:"-"`
	SourceOther  string    `json:"-"`
	SourceDate   time.Time `json:"sourceDate"`
	UploadDate   time.Time `json:"uploadDate"`
	OriginalDate time.Time `json:"originalDate"`

	FileName    string `json:"fileName"`
	Title       string `json:"title"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`

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
}

type PhotoFilter struct {
	Private     bool
	CameraModel string
}

type PhotoOrder int

const (
	None PhotoOrder = iota
	UploadDate
	OriginalDate
)

type Range struct {
	Offset int
	Limit  int
}

type Reaction struct {
	GuestId uuid.UUID
	PhotoId uuid.UUID
	Kind    string
}

const SourceGoogle = "gdrive"
const SourceLocal = "local"

type User struct {
	Name            string `json:"name"`
	Bio             string `json:"bio"`
	Pic             string `json:"pic"`
	DriveFolderId   string `json:"driveFolderId,omitempty"`
	DriveFolderName string `json:"driveFolderName,omitempty"`
	Config          string `json:"config,omitempty"`
}
