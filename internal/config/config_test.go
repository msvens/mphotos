package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestConfigFile(t *testing.T) {
	if err := testConfig(); err != nil {
		t.Errorf("could not create config %v", err)
	}

	//db config:
	if DbHost() != "localhost" {
		t.Errorf("expected localhost got %v", DbHost())
	}
	if DbPort() != 5432 {
		t.Errorf("expected 5432 got %v", DbPort())
	}
	if DbUser() != "user" {
		t.Errorf("expected user got %v", DbUser())
	}
	if DbPassword() != "password" {
		t.Errorf("expected password got %v", DbPassword())
	}
	if DbName() != "name" {
		t.Errorf("expected name got %v", DbName())
	}

	//server config:
	if ServerPort() != 8050 {
		t.Errorf("expected 8050 got %v", ServerPort())
	}
	if ServerPrefix() != "/api" {
		t.Errorf("expected /api got %v", ServerPrefix())
	}
	if ServerHost() != "localhost" {
		t.Errorf("expected localhost got %v", ServerHost())
	}
	if ServerAddr() != "localhost:8050" {
		t.Errorf("expected localhost:8050 got %v", ServerAddr())
	}
	if VerifyUrl() != "http://localhost:8050/api/guest/verify" {
		t.Errorf("expected http://localhost:8050/api/guest/verify got #{VerifyUrl()}")
	}
	if !SecureCookies() {
		t.Errorf("expected secureCookies true got %v", SecureCookies())
	}

	//session config:
	if SessionCookieName() != "server-session" {
		t.Errorf("expected server-session got %v", SessionCookieName())
	}

	if SessionAuthcKey() != "authKey" {
		t.Errorf("expected authKey got %v", SessionAuthcKey())
	}

	if SessionAuthcKeyOld() != "authKeyOld" {
		t.Errorf("expected authKeyOld got %v", SessionAuthcKeyOld())
	}

	if SessionEncKey() != "encKey" {
		t.Errorf("expected encKey got %v", SessionEncKey())
	}

	if SessionEncKeyOld() != "encKeyOld" {
		t.Errorf("expected encKeyOld got %v", SessionEncKeyOld())
	}

	//service config:
	if ServiceRoot() != ".server" {
		t.Errorf("expected .server got %v", ServiceRoot())
	}
	//auth config:
	if AuthMethod() != "password" {
		t.Errorf("expected password got %v", AuthMethod())
	}
	if AuthPassword() != "password" {
		t.Errorf("expected password got %v", AuthPassword())
	}
	allowed := AuthAllowedEmails()
	if len(allowed) != 1 || allowed[0] != "msvens@gmail.com" {
		t.Errorf("expected [msvens@gmail.com] got %v", allowed)
	}
	if AuthGoogleLoginRedirectUrl() != "http://localhost:8050/api/auth/login/callback" {
		t.Errorf("unexpected login redirect url: %v", AuthGoogleLoginRedirectUrl())
	}
	if GuestGoogleRedirectUrl() != "http://localhost:8050/api/auth/guest/callback" {
		t.Errorf("unexpected guest redirect url: %v", GuestGoogleRedirectUrl())
	}
	//google config:
	if GoogleClientId() != "clientId" {
		t.Errorf("expected clientId got %v", GoogleClientId())
	}
	if GoogleClientSecret() != "clientSecret" {
		t.Errorf("expected clientSecret got %v", GoogleClientSecret())
	}
	if GoogleRedirectUrl() != "http://some/redirect/url" {
		t.Errorf("expected http://some/redirect/url got %v", GoogleRedirectUrl())
	}

}

// TestEnvOverridesSecrets covers the variables a systemd EnvironmentFile sets in
// production: each must override config.yaml, and an unset or empty variable
// must fall back to the file so an existing config.yaml keeps working.
func TestEnvOverridesSecrets(t *testing.T) {
	if err := testConfig(); err != nil {
		t.Fatalf("could not create config %v", err)
	}
	cases := []struct {
		env      string
		get      func() string
		fromFile string
	}{
		{"MPHOTOS_DB_PASSWORD", DbPassword, "password"},
		{"MPHOTOS_AUTH_PASSWORD", AuthPassword, "password"},
		{"MPHOTOS_GOOGLE_CLIENTSECRET", GoogleClientSecret, "clientSecret"},
		{"MPHOTOS_SESSION_AUTHKEY", SessionAuthcKey, "authKey"},
		{"MPHOTOS_SESSION_ENCKEY", SessionEncKey, "encKey"},
	}
	for _, c := range cases {
		t.Run(c.env, func(t *testing.T) {
			if got := c.get(); got != c.fromFile {
				t.Errorf("unset: expected file value %q got %q", c.fromFile, got)
			}
			t.Setenv(c.env, "")
			if got := c.get(); got != c.fromFile {
				t.Errorf("empty: expected file value %q got %q", c.fromFile, got)
			}
			t.Setenv(c.env, "from-env")
			if got := c.get(); got != "from-env" {
				t.Errorf("set: expected from-env got %q", got)
			}
		})
	}
}

func TestCheckSecrets(t *testing.T) {
	if err := testConfig(); err != nil {
		t.Fatalf("could not create config %v", err)
	}
	if err := CheckSecrets(); err != nil {
		t.Fatalf("example config should pass: %v", err)
	}
	set := func(key string, v interface{}) {
		old := viper.Get(key)
		viper.Set(key, v)
		t.Cleanup(func() { viper.Set(key, old) })
	}

	set("auth.password", "")
	set("session.encKey", "")
	err := CheckSecrets()
	if err == nil {
		t.Fatal("expected empty auth.password and session.encKey to fail")
	}
	for _, k := range []string{"auth.password", "session.encKey"} {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("expected error to name %s, got %v", k, err)
		}
	}

	// Google login has no password, so an empty auth.password is fine there.
	set("auth.method", "google")
	set("session.encKey", "encKey")
	if err := CheckSecrets(); err != nil {
		t.Errorf("google method: expected no error, got %v", err)
	}
}
