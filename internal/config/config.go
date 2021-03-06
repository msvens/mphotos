package config

import (
	"github.com/spf13/viper"
	"path/filepath"
)

var configed bool = false

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
	configed = true
	return err
}

func testConfig() error {
	viper.SetConfigName("config_example")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../..")
	err := viper.ReadInConfig()
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

func ServiceImgDir() string {
	return viper.GetString("service.imgDir")
}

func ServicePassword() string {
	return viper.GetString("service.password")
}

func ServiceThumbDir() string {
	return viper.GetString("service.thumbDir")
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
