package server

import (
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"go.uber.org/zap"
	"html/template"
	"net/http"
	"strings"
	"time"
)

type SessionGuest struct {
	Id string `json:"id"`
}

const (
	Session_Month int = 60 * 60 * 24 * 30

	// guestCookieMaxAge is how long a guest stays signed in before the cookie
	// expires and an email one-time-code login is required again.
	guestCookieMaxAge = Session_Month
	// signupTokenMaxAge is the window to click the emailed verification link, and
	// also the age past which an unverified guest is reaped.
	signupTokenMaxAge = 60 * 60 * 24 // 24 hours
	// loginCodeMaxAge is the lifetime of an emailed numeric login code.
	loginCodeMaxAge = 15 * 60 // 15 minutes
)

var emptyuuid = uuid.UUID{}

type WelcomeEmail struct {
	VerifyUrl string
	Code      string
	Name      string
}

type LoginCodeEmail struct {
	Name string
	Code string
}

var templates = template.Must(template.ParseFiles(
	"tmpl/welcome-email.html",
	"tmpl/login-code-email.html",
))

func sessionGuest(session *sessions.Session) (SessionGuest, bool) {
	val := session.Values["guest"]
	guest, ok := val.(SessionGuest)
	return guest, ok
}

func guestUUID(w http.ResponseWriter, r *http.Request, s *mserver) (uuid.UUID, error) {
	session, err := s.store.Get(r, s.guestCookie)
	if err != nil {
		s.l.Warnw("could not get session", zap.Error(err))
		return s.clearGuestCookie(w, r, session)
	}
	guest, ok := sessionGuest(session)

	if !ok {
		return emptyuuid, nil
	}

	if uuid, err := uuid.Parse(guest.Id); err != nil {
		s.l.Warnw("could not parse session guest")
		return s.clearGuestCookie(w, r, session)
	} else {
		if s.pg.Guest.HasVerified(uuid) {
			return uuid, nil
		} else {
			s.l.Debugw("guest not found or not verified")
			return s.clearGuestCookie(w, r, session)
		}
	}
}

func (s *mserver) clearGuestCookie(w http.ResponseWriter, r *http.Request, session *sessions.Session) (uuid.UUID, error) {
	session.Values["guest"] = SessionGuest{}
	session.Options.MaxAge = -1
	return emptyuuid, session.Save(r, w)
}

func (s *mserver) saveGuestCookie(w http.ResponseWriter, r *http.Request, guest uuid.UUID, days int) error {
	session, _ := s.store.Get(r, s.guestCookie)
	gid := &SessionGuest{guest.String()}
	s.l.Debugw("saving guest cookie", "guestId", gid.Id)
	session.Values["guest"] = gid
	session.Options.MaxAge = days
	if err := session.Save(r, w); err != nil {
		return InternalError(err.Error())
	}
	return nil
}

func (s *mserver) handleCommentPhoto(r *http.Request, uid uuid.UUID) (interface{}, error) {
	if photoId, err := uuid.Parse(Var(r, "img")); err != nil {
		return nil, BadRequestError("Could not parse img id")
	} else {
		type request struct {
			Body string
		}
		var params request
		if err := decodeRequest(r, &params); err != nil {
			return nil, err
		}
		return s.pg.Comment.Add(uid, photoId, params.Body)
	}
}

func (s *mserver) handlePhotoComments(r *http.Request, loggedIn bool) (interface{}, error) {
	if photoId, err := uuid.Parse(Var(r, "img")); err != nil {
		return nil, BadRequestError("Could not parse img id")
	} else {
		type resp struct {
			Id          int       `json:"id"`
			Name        string    `json:"name"`
			Description string    `json:"description"`
			PhotoId     uuid.UUID `json:"photoId"`
			Time        time.Time `json:"time"`
			Body        string    `json:"body"`
		}
		comments, err := s.pg.Comment.ListByPhoto(photoId)
		if err != nil {
			return nil, err
		}
		ret := []*resp{}
		for _, c := range comments {
			u, _ := s.pg.Guest.Get(c.GuestId)
			ret = append(ret, &resp{Id: c.Id, Name: u.Name, Description: u.Description, PhotoId: c.PhotoId, Time: c.Time, Body: c.Body})
		}
		return ret, nil
	}
}

func (s *mserver) handleGuest(r *http.Request, uuid uuid.UUID) (interface{}, error) {
	return s.pg.Guest.Get(uuid)
}

// handleGuests lists all guests for the owner. It is authOnly, so it may return
// the owner-only fields (full name, email) that public responses never expose.
func (s *mserver) handleGuests(r *http.Request) (interface{}, error) {
	return s.pg.Guest.List()
}

func (s *mserver) handleLikePhoto(r *http.Request, guestId uuid.UUID) (interface{}, error) {
	if photoId, err := uuid.Parse(Var(r, "photoid")); err != nil {
		return nil, BadRequestError("Could not parse img id")
	} else {
		if s.pg.Photo.Has(photoId) {
			return photoId, s.pg.Reaction.Add(&dao.Reaction{GuestId: guestId, PhotoId: photoId, Kind: "like"})
		} else {
			return nil, NotFoundError("img not found")
		}
	}
}

func (s *mserver) handleUnlikePhoto(r *http.Request, guestId uuid.UUID) (interface{}, error) {
	var photoId uuid.UUID
	if err := uid(r, "photoid", &photoId); err != nil {
		return nil, err
	}
	if s.pg.Photo.Has(photoId) {
		return photoId, s.pg.Reaction.Delete(&dao.Reaction{GuestId: guestId, PhotoId: photoId, Kind: "like"})
	} else {
		return nil, NotFoundError("img not found")
	}
}

func (s *mserver) handlePhotoLikes(r *http.Request, loggedIn bool) (interface{}, error) {
	if photoId, err := uuid.Parse(Var(r, "photoid")); err != nil {
		return nil, BadRequestError("Could not parse img id")
	} else {
		return s.pg.Reaction.ListByPhoto(photoId)
	}
}

// handleVerifyGuest activates a guest from the one-time signup token in their
// verification link, then signs them in. The token (unlike the old scheme, which
// reused the guest's own uuid) is unguessable, single-use, and expires.
func (s *mserver) handleVerifyGuest(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	type request struct {
		Code string
	}
	var params request
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	if params.Code == "" {
		return nil, BadRequestError("missing verification code")
	}
	id, ok, err := s.pg.GuestCode.FindSignup(params.Code)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, BadRequestError("invalid or expired verification link")
	}
	ver, err := s.pg.Guest.Verify(id)
	if err != nil {
		return nil, err
	}
	_ = s.pg.GuestCode.Delete(id, dao.GuestCodeSignup)
	return ver, s.saveGuestCookie(w, r, id, guestCookieMaxAge)
}

func (s *mserver) handleIsGuest(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var guest = ctxGuest(r.Context())
	s.l.Debugw("handleIsGuest", "guestId", guest)
	if guest == emptyuuid {
		return AuthUser{false}, nil
	} else {
		return AuthUser{true}, nil
	}
}

func (s *mserver) handleGuestLikes(r *http.Request, guestId uuid.UUID) (interface{}, error) {
	if likes, err := s.pg.Reaction.ListByGuest(guestId); err != nil {
		return nil, err
	} else {
		return likes, nil
	}
}

func (s *mserver) handleLogoutGuest(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return AuthUser{false}, s.saveGuestCookie(w, r, emptyuuid, -1)
}

func (s *mserver) handleGuestLikePhoto(r *http.Request, guestId uuid.UUID) (interface{}, error) {
	if photoId, err := uuid.Parse(Var(r, "photoid")); err != nil {
		return nil, BadRequestError("Could not parse img id")
	} else {
		type ret struct {
			Like bool `json:"like"`
		}
		return ret{s.pg.Reaction.Has(guestId, photoId)}, nil
	}
}

// sendSignupEmail issues a fresh single-use signup token, stores it, and emails
// the verification link.
func (s *mserver) sendSignupEmail(guestId uuid.UUID, name, email string) error {
	if s.ms == nil {
		return InternalError("email service not configured")
	}
	token, err := generateOAuthState()
	if err != nil {
		return InternalError(err.Error())
	}
	expires := time.Now().Add(time.Duration(signupTokenMaxAge) * time.Second)
	if err := s.pg.GuestCode.Issue(guestId, dao.GuestCodeSignup, token, expires); err != nil {
		return err
	}
	var b strings.Builder
	we := WelcomeEmail{Name: name, Code: token, VerifyUrl: config.VerifyUrl()}
	if err := templates.ExecuteTemplate(&b, "welcome-email.html", we); err != nil {
		return err
	}
	_, err = s.ms.SendHtmlMessage(email, "Mellowtech Guest Verification", b.String())
	return err
}

// handleCreateGuest registers a guest. The guest is created pending and is only
// activated (and given a cookie) once they click the emailed verification link —
// no cookie is issued here.
func (s *mserver) handleCreateGuest(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	type request struct {
		Email       string
		Name        string
		FullName    string
		Description string
	}
	var params request
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Email) == "" || strings.TrimSpace(params.Name) == "" {
		return nil, BadRequestError("name and email are required")
	}

	// Returning email: verified guests should log in; unverified (never clicked)
	// guests get their pending signup refreshed and a new link.
	if s.pg.Guest.HasByEmail(params.Email) {
		existing, err := s.pg.Guest.GetByEmail(params.Email)
		if err != nil {
			return nil, err
		}
		if existing.Verified {
			return nil, BadRequestError("an account with this email already exists — please log in")
		}
		if _, err := s.pg.Guest.UpdateProfile(existing.Id, params.Name, params.FullName, params.Description); err != nil {
			return nil, err
		}
		if err := s.sendSignupEmail(existing.Id, params.Name, params.Email); err != nil {
			return nil, err
		}
		return existing, nil
	}

	if s.pg.Guest.HasByName(params.Name) {
		return nil, BadRequestError("name already taken")
	}
	guest, err := s.pg.Guest.Add(params.Name, params.Email)
	if err != nil {
		return nil, err
	}
	if params.FullName != "" || params.Description != "" {
		if _, err := s.pg.Guest.UpdateProfile(guest.Id, params.Name, params.FullName, params.Description); err != nil {
			_ = s.pg.Guest.Delete(guest.Id)
			return nil, err
		}
	}
	if err := s.sendSignupEmail(guest.Id, params.Name, params.Email); err != nil {
		_ = s.pg.Guest.Delete(guest.Id)
		return nil, err
	}
	return guest, nil
}

// handleGuestLogin starts an email one-time-code login: a verified guest with
// this email is emailed a numeric code. The response is deliberately neutral so
// it never reveals whether an email is registered.
func (s *mserver) handleGuestLogin(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	type request struct {
		Email string
	}
	var params request
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	neutral := map[string]string{"message": "if that email is registered, a login code has been sent"}
	if strings.TrimSpace(params.Email) == "" || !s.pg.Guest.HasByEmail(params.Email) {
		return neutral, nil
	}
	guest, err := s.pg.Guest.GetByEmail(params.Email)
	if err != nil || !guest.Verified {
		return neutral, nil
	}
	if s.ms == nil {
		return nil, InternalError("email service not configured")
	}
	code, err := generateLoginCode()
	if err != nil {
		return nil, InternalError(err.Error())
	}
	expires := time.Now().Add(time.Duration(loginCodeMaxAge) * time.Second)
	if err := s.pg.GuestCode.Issue(guest.Id, dao.GuestCodeLogin, code, expires); err != nil {
		return nil, err
	}
	var b strings.Builder
	if err := templates.ExecuteTemplate(&b, "login-code-email.html", LoginCodeEmail{Name: guest.Name, Code: code}); err != nil {
		return nil, err
	}
	if _, err := s.ms.SendHtmlMessage(guest.Email, "Your Mellowtech Photos login code", b.String()); err != nil {
		return nil, err
	}
	return neutral, nil
}

// handleGuestLoginVerify consumes an email login code and signs the guest in.
func (s *mserver) handleGuestLoginVerify(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	type request struct {
		Email string
		Code  string
	}
	var params request
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	guest, err := s.pg.Guest.GetByEmail(params.Email)
	if err != nil {
		return nil, UnauthorizedError("invalid email or code")
	}
	ok, err := s.pg.GuestCode.ConsumeLogin(guest.Id, params.Code)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, UnauthorizedError("invalid email or code")
	}
	return guest, s.saveGuestCookie(w, r, guest.Id, guestCookieMaxAge)
}

// reapGuests periodically removes guests who never verified within the signup
// window (cascading their comments/likes) and purges expired one-time codes.
func (s *mserver) reapGuests() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-time.Duration(signupTokenMaxAge) * time.Second)
		if n, err := s.pg.Guest.DeleteUnverifiedBefore(cutoff); err != nil {
			s.l.Warnw("guest reaper: could not delete unverified guests", zap.Error(err))
		} else if n > 0 {
			s.l.Infow("guest reaper: removed unverified guests", "count", n)
		}
		if n, err := s.pg.GuestCode.DeleteExpiredBefore(time.Now()); err != nil {
			s.l.Warnw("guest reaper: could not delete expired codes", zap.Error(err))
		} else if n > 0 {
			s.l.Debugw("guest reaper: removed expired codes", "count", n)
		}
	}
}

func (s *mserver) handleUpdateGuest(r *http.Request, guestId uuid.UUID) (interface{}, error) {
	type request struct {
		Name        string
		FullName    string
		Description string
	}
	var params request
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Name) == "" {
		return nil, BadRequestError("name cannot contain only white space characters")
	}
	// Email is the login identity and is intentionally not updatable here.
	return s.pg.Guest.UpdateProfile(guestId, params.Name, params.FullName, params.Description)
}
