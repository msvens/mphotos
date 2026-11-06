package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
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
const AvatarDir = "avatar"

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
		return fmt.Errorf("no serviceroot defined")
	}
	photoTypePaths = make(map[PhotoType]string)
	for k, v := range photoTypeDirNames {
		photoTypePaths[k] = ServicePath(v)
	}
	return nil
}

// bindEnv lets MPHOTOS_* environment variables override the config file: the key
// upper-cased with "." as "_" (db.password -> MPHOTOS_DB_PASSWORD,
// google.clientSecret -> MPHOTOS_GOOGLE_CLIENTSECRET). An unset or empty variable
// falls back to the file, so secrets can move out of config.yaml gradually.
func bindEnv() {
	viper.SetEnvPrefix("MPHOTOS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
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
	bindEnv()

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
	bindEnv()
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
	bindEnv()
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
	return fmt.Sprintf("%s:%d", ServerHost(), ServerPort())
}

func VerifyUrl() string {
	return viper.GetString("server.verifyUrl")
}

// SecureCookies reports whether session cookies should carry the Secure flag.
// True in production (HTTPS behind nginx); false for local dev over http.
func SecureCookies() bool {
	return viper.GetBool("server.secureCookies")
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

func AvatarPath() string {
	return filepath.Join(ServiceRoot(), AvatarDir)
}

func AvatarFilePath(fname string) string {
	return filepath.Join(AvatarPath(), fname)
}

func AuthMethod() string {
	m := viper.GetString("auth.method")
	if m == "" {
		return "password"
	}
	return m
}

func AuthPassword() string {
	return viper.GetString("auth.password")
}

// CheckSecrets reports the secrets the server cannot run safely without. Once
// secrets live in an env file, a missing or misnamed variable silently reads as
// "" — an empty auth.password would let an empty login in. db.password is not
// checked: peer/trust auth legitimately has none.
func CheckSecrets() error {
	var missing []string
	if AuthMethod() == "password" && AuthPassword() == "" {
		missing = append(missing, "auth.password")
	}
	if SessionAuthcKey() == "" {
		missing = append(missing, "session.authKey")
	}
	if SessionEncKey() == "" {
		missing = append(missing, "session.encKey")
	}
	if len(missing) > 0 {
		return fmt.Errorf("empty required config: %s (set in config.yaml or MPHOTOS_* env)",
			strings.Join(missing, ", "))
	}
	return nil
}

func AuthAllowedEmails() []string {
	return viper.GetStringSlice("auth.google.allowedEmails")
}

func AuthGoogleLoginRedirectUrl() string {
	return viper.GetString("auth.google.loginRedirectUrl")
}

func AuthGoogleUIRedirectPath() string {
	p := viper.GetString("auth.google.uiRedirectPath")
	if p == "" {
		return "/"
	}
	return p
}

// GuestGoogleRedirectUrl is the OAuth callback URL for the guest Google login
// flow. It must be a distinct path from the owner login callback and registered
// as an authorized redirect URI in the Google console.
func GuestGoogleRedirectUrl() string {
	return viper.GetString("auth.google.guestRedirectUrl")
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
