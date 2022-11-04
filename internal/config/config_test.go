package config

import "testing"

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
	if ServicePassword() != "password" {
		t.Errorf("expected password got %v", ServicePassword())
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
