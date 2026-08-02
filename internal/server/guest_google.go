package server

import (
	"bytes"
	"fmt"
	"github.com/google/uuid"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"go.uber.org/zap"
	"net/http"
	"net/url"
	"strings"
)

func (s *mserver) guestLoginStateCookieName() string {
	return s.cookieName + "-guest-login-state"
}

// handleGuestGoogleRedirect starts the guest Google login: it stores a single-use
// CSRF state in a dedicated cookie and redirects to Google. Mirrors the owner
// login redirect but with the guest OAuth config and no allowlist.
func (s *mserver) handleGuestGoogleRedirect(w http.ResponseWriter, r *http.Request) {
	state, err := generateOAuthState()
	if err != nil {
		s.l.Errorw("could not generate guest oauth state", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	stateSession, _ := s.store.Get(r, s.guestLoginStateCookieName())
	stateSession.Values["state"] = state
	stateSession.Options.MaxAge = oauthLoginStateMaxAge
	if err := stateSession.Save(r, w); err != nil {
		s.l.Errorw("could not save guest oauth state cookie", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, s.gconfigGuestLogin.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// handleGuestGoogleCallback completes the guest Google login: validates the
// single-use state, exchanges the code, fetches the verified profile, resolves
// it to a guest (link/create), and issues the guest cookie. Unlike the owner
// flow there is no email allowlist — any verified Google account may be a guest.
func (s *mserver) handleGuestGoogleCallback(w http.ResponseWriter, r *http.Request) {
	redirTo := func(errCode string) {
		target := config.AuthGoogleUIRedirectPath()
		if errCode != "" {
			sep := "?"
			if strings.Contains(target, "?") {
				sep = "&"
			}
			target = target + sep + "error=" + url.QueryEscape(errCode)
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	}

	queryState := r.FormValue("state")
	stateSession, _ := s.store.Get(r, s.guestLoginStateCookieName())
	expected, _ := stateSession.Values["state"].(string)
	// Always expire the state cookie after first use, regardless of outcome.
	stateSession.Options.MaxAge = -1
	_ = stateSession.Save(r, w)

	if queryState == "" || expected == "" || queryState != expected {
		s.l.Info("guest oauth state mismatch")
		redirTo("invalid_state")
		return
	}
	code := r.FormValue("code")
	if code == "" {
		redirTo("invalid_state")
		return
	}

	token, err := s.gconfigGuestLogin.Exchange(r.Context(), code)
	if err != nil {
		s.l.Errorw("guest login code exchange failed", zap.Error(err))
		redirTo("exchange_failed")
		return
	}
	info, err := fetchGoogleUserinfo(r.Context(), token)
	if err != nil {
		s.l.Errorw("guest userinfo fetch failed", zap.Error(err))
		redirTo("exchange_failed")
		return
	}
	if !info.EmailVerified || info.Email == "" || info.Sub == "" {
		s.l.Infow("guest google account not usable", "email", info.Email, "verified", info.EmailVerified)
		redirTo("unauthorized_email")
		return
	}

	guest, err := s.provisionGoogleGuest(info)
	if err != nil {
		s.l.Errorw("could not provision google guest", zap.Error(err))
		redirTo("exchange_failed")
		return
	}
	if err := s.saveGuestCookie(w, r, guest.Id, guestCookieMaxAge); err != nil {
		s.l.Errorw("could not save guest cookie", zap.Error(err))
		redirTo("exchange_failed")
		return
	}
	redirTo("")
}

// provisionGoogleGuest resolves a verified Google profile to a guest: an existing
// linked guest (by Google id), an existing email-OTP guest (linked by verified
// email — same person), or a brand-new guest. A newly created guest gets the
// Google picture imported as their avatar, best-effort.
func (s *mserver) provisionGoogleGuest(info *googleUserinfo) (*dao.Guest, error) {
	// 1. Already linked by Google id.
	if g, err := s.pg.Guest.GetByGoogleId(info.Sub); err == nil {
		return g, nil
	}
	// 2. Existing guest with the same (Google-verified) email → link, and verify
	//    if they were still pending.
	if s.pg.Guest.HasByEmail(info.Email) {
		g, err := s.pg.Guest.GetByEmail(info.Email)
		if err != nil {
			return nil, err
		}
		if !g.Verified {
			if _, err := s.pg.Guest.Verify(g.Id); err != nil {
				return nil, err
			}
		}
		linked, err := s.pg.Guest.SetGoogleId(g.Id, info.Sub)
		if err != nil {
			return nil, err
		}
		// Fill an empty avatar from the Google picture; never clobber one the
		// guest has already set. They can remove it afterwards if they like.
		if linked.Avatar == "" {
			s.importGoogleAvatar(linked.Id, info.Picture)
			return s.pg.Guest.Get(linked.Id)
		}
		return linked, nil
	}
	// 3. New guest.
	displayName := googleDisplayName(info)
	name := s.dedupGuestName(displayName)
	g, err := s.pg.Guest.Add(name, info.Email)
	if err != nil {
		return nil, err
	}
	if _, err := s.pg.Guest.UpdateProfile(g.Id, name, displayName, ""); err != nil {
		_ = s.pg.Guest.Delete(g.Id)
		return nil, err
	}
	if _, err := s.pg.Guest.Verify(g.Id); err != nil {
		_ = s.pg.Guest.Delete(g.Id)
		return nil, err
	}
	if _, err := s.pg.Guest.SetGoogleId(g.Id, info.Sub); err != nil {
		_ = s.pg.Guest.Delete(g.Id)
		return nil, err
	}
	s.importGoogleAvatar(g.Id, info.Picture)
	return s.pg.Guest.Get(g.Id)
}

// dedupGuestName returns name, or name-2/name-3/... if the nickname is taken.
// A redirect flow can't re-prompt, so collisions are resolved automatically.
func (s *mserver) dedupGuestName(name string) string {
	if !s.pg.Guest.HasByName(name) {
		return name
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", name, i)
		if !s.pg.Guest.HasByName(candidate) {
			return candidate
		}
	}
}

// importGoogleAvatar downloads a guest's Google profile picture into their
// avatar. Best-effort: any failure is logged and ignored so it never blocks
// login. Reuses the SSRF-guarded fetch + resize pipeline from the avatar upload.
func (s *mserver) importGoogleAvatar(guestId uuid.UUID, pictureURL string) {
	if pictureURL == "" {
		return
	}
	data, err := fetchRemoteImage(pictureURL)
	if err != nil {
		s.l.Infow("could not fetch google avatar", "error", err)
		return
	}
	ext, err := storeGuestAvatar(guestId, bytes.NewReader(data))
	if err != nil {
		s.l.Infow("could not store google avatar", "error", err)
		return
	}
	if _, err := s.pg.Guest.UpdateAvatar(ext, guestId); err != nil {
		s.l.Warnw("could not record google avatar", "error", err)
	}
}

// googleDisplayName picks a nickname from a Google profile: the display name, or
// the email local-part as a fallback.
func googleDisplayName(info *googleUserinfo) string {
	if n := strings.TrimSpace(info.Name); n != "" {
		return n
	}
	if at := strings.IndexByte(info.Email, '@'); at > 0 {
		return info.Email[:at]
	}
	return "guest"
}
