package server

import (
	"net/http"
	"testing"
)

// TestRoutesNoConflict registers the whole routing table and fails if
// net/http.ServeMux rejects any pair of patterns as ambiguous. Go 1.22 panics at
// registration when neither of two overlapping patterns is more specific (e.g. a
// wildcard-then-literal path like /guest/{guestid}/avatar crossing a
// literal-then-wildcard one like /guest/likes/{photoid}). routes() only touches
// the mux and prefix, so this needs no database.
func TestRoutesNoConflict(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("route registration panicked on conflicting patterns: %v", r)
		}
	}()
	s := &mserver{r: http.NewServeMux(), prefixPath: "/api"}
	s.routes()
}
