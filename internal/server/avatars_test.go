package server

import "testing"

// TestValidatePublicURL covers the SSRF guard. IP-literal hosts are used so
// net.LookupIP resolves them without any DNS/network access.
func TestValidatePublicURL(t *testing.T) {
	rejected := []string{
		"ftp://8.8.8.8/x",       // non-http(s) scheme
		"file:///etc/passwd",    // non-http(s) scheme
		"http://127.0.0.1/x",    // loopback
		"http://[::1]/x",        // loopback (v6)
		"http://10.1.2.3/x",     // private
		"http://192.168.0.5/x",  // private
		"http://172.16.9.9/x",   // private
		"http://169.254.10.1/x", // link-local
		"http://0.0.0.0/x",      // unspecified
		"http:///nohost",        // no host
		"://bad",                // unparseable / no scheme
	}
	for _, u := range rejected {
		if err := validatePublicURL(u); err == nil {
			t.Errorf("expected %q to be rejected", u)
		}
	}

	allowed := []string{
		"http://8.8.8.8/avatar.jpg",
		"https://1.1.1.1/pic.png",
	}
	for _, u := range allowed {
		if err := validatePublicURL(u); err != nil {
			t.Errorf("expected %q to be allowed, got %v", u, err)
		}
	}
}
