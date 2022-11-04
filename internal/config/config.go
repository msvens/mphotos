package config

import (
	"fmt"
	"github.com/spf13/viper"
	"path/filepath"
)

var configed bool = false

type PhotoType int

const (
	Original PhotoType = iota
	Thumb
	Landscape
	Square
	Portrait
	Resize
)

const CameraDir = "camera"

var photoTypeDirNames = map[PhotoType]string{
	Original:  "img",
	Thumb:     "thumb",
	Landscape: "landscape",
	Square:    "square",
	Portrait:  "portrait",
	Resize:    "resize",
}

func (pt PhotoType) String() string {
	return photoTypeDirNames[pt]
}

var photoTypePaths map[PhotoType]string

func setPhotoTypePaths() error {
	if ServiceRoot() == "" {
		return fmt.Errorf("No Serviceroot defined")
	}
	photoTypePaths = make(map[PhotoType]string)
	for k, v := range photoTypeDirNames {
		photoTypePaths[k] = ServicePath(v)
	}
	return nil
}

func InitConfig() error {
	if configed {
		return nil
	}
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.mphotos")
	viper.AddConfigPath("/etc/mphotos")
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()
	if err == nil {
		err = setPhotoTypePaths()
	}
	configed = true
	return err
}

func NewConfig(name string) error {
	viper.SetConfigName(name)
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.mphotos")
	viper.AddConfigPath("/etc/mphotos")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../..")
	err := viper.ReadInConfig()
	if err == nil {
		err = setPhotoTypePaths()
	}
	configed = true
	return err
}

func testConfig() error {
	viper.SetConfigName("config_example")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../..")
	err := viper.ReadInConfig()
	if err == nil {
		err = setPhotoTypePaths()
	}
	configed = true
	return err
}

func DbHost() string {
	return viper.GetString("db.host")
}

func DbName() string {
	return viper.GetString("db.name")
}

func DbPassword() string {
	return viper.GetString("db.password")
}

func DbPort() int {
	return viper.GetInt("db.port")
}

func DbUser() string {
	return viper.GetString("db.user")
}

func GoogleClientId() string {
	return viper.GetString("google.clientId")
}

func GoogleRedirectUrl() string {
	return viper.GetString("google.redirectUrl")
}

func GoogleClientSecret() string {
	return viper.GetString("google.ClientSecret")
}

func ServerPort() int {
	return viper.GetInt("server.port")
}

func ServerHost() string {
	return viper.GetString("server.host")
}

func ServerAddr() string {
    return fmt.Sprintf("%s:%d",ServerHost(),ServerPort())
}

func VerifyUrl() string {
	return viper.GetString("server.verifyUrl")
}

func ServerPrefix() string {
	return viper.GetString("server.prefix")
}

func ServiceRoot() string {
	return viper.GetString("service.root")
}

func ServicePath(fileName string) string {
	return filepath.Join(ServiceRoot(), fileName)
}

func PhotoPath(pt PhotoType) string {
	return photoTypePaths[pt]
}

func PhotoPaths() map[PhotoType]string {
	return photoTypePaths
}

func PhotoFilePath(pt PhotoType, fname string) string {
	return filepath.Join(PhotoPath(pt), fname)
}

func CameraPath() string {
	return filepath.Join(ServiceRoot(), CameraDir)
}

func CameraFilePath(fname string) string {
	return filepath.Join(CameraPath(), fname)
}

func ServicePassword() string {
	return viper.GetString("service.password")
}

func SessionAuthcKey() string {
	return viper.GetString("session.authKey")
}

func SessionCookieName() string {
	return viper.GetString("session.cookieName")
}

func SessionEncKey() string {
	return viper.GetString("session.encKey")
}

func SessionAuthcKeyOld() string {
	return viper.GetString("session.authKeyOld")
}

func SessionEncKeyOld() string {
	return viper.GetString("session.encKeyOld")
}
