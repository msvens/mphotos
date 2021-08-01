package server

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/model"
	"html/template"
	"net/http"
	"strings"
)

type SessionGuest struct {
	Id string `json:"id"`
}

const (
	Session_Year  int = 60 * 60 * 24 * 365
	Session_Month int = 60 * 60 * 24 * 30
)

var emptyuuid = uuid.UUID{}

type WelcomeEmail struct {
	VerifyUrl string
	Code      string
	Name      string
}

var templates = template.Must(template.ParseFiles("tmpl/welcome-email.html"))

func sessionGuest(session *sessions.Session) (SessionGuest, bool) {
	val := session.Values["guest"]
	guest, ok := val.(SessionGuest)
	return guest, ok
}

func guestUUID(w http.ResponseWriter, r *http.Request, s *mserver) (uuid.UUID, error) {
	session, err := s.store.Get(r, s.guestCookie)
	if err != nil {
		fmt.Println("Could not get session ", err)
		return s.clearGuestCookie(w, r, session)
	}
	guest, ok := sessionGuest(session)

	if !ok {
		return emptyuuid, nil
	}

	if uuid, err := uuid.Parse(guest.Id); err != nil {
		fmt.Println("Could not parse session guest 1", ok)
		return s.clearGuestCookie(w, r, session)
	} else {
		if s.db.HasGuest(uuid) {
			return uuid, nil
		} else {
			fmt.Println("Did not have guest ", ok)
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
	session.Values["guest"] = gid
	session.Options.MaxAge = days
	if err := session.Save(r, w); err != nil {
		return InternalError(err.Error())
	}
	return nil
}

func (s *mserver) handleCommentPhoto(r *http.Request, uuid uuid.UUID) (interface{}, error) {
	photo := Var(r, "photo")

	type request struct {
		Body string
	}
	var params request
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	return s.db.AddComment(uuid, photo, params.Body)
}

func (s *mserver) handlePhotoComments(r *http.Request, loggedIn bool) (interface{}, error) {
	photo := Var(r, "photo")
	if s.db.HasPhoto(photo, loggedIn) {
		return s.db.PhotoComments(photo)
	} else {
		return nil, NotFoundError("photo not found")
	}
}

func (s *mserver) handleGuest(r *http.Request, uuid uuid.UUID) (interface{}, error) {
	type resp struct {
		Name  string
		Email string
	}

	return s.db.Guest(uuid)
}

func (s *mserver) handleLikePhoto(r *http.Request, uuid uuid.UUID) (interface{}, error) {
	photo := Var(r, "photo")
	if s.db.HasPhoto(photo, false) {
		if err := s.db.AddLike(uuid, photo); err != nil {
			return nil, err
		} else {
			return photo, nil
		}
	} else {
		return nil, NotFoundError("photo not found")
	}
}

func (s *mserver) handleUnlikePhoto(r *http.Request, uuid uuid.UUID) (interface{}, error) {
	photo := Var(r, "photo")
	if s.db.HasPhoto(photo, false) {
		if err := s.db.DeleteLike(uuid, photo); err != nil {
			return nil, err
		} else {
			return photo, nil
		}
	} else {
		return nil, NotFoundError("photo not found")
	}
}

func (s *mserver) handlePhotoLikes(r *http.Request, loggedIn bool) (interface{}, error) {
	photo := Var(r, "photo")
	if s.db.HasPhoto(photo, loggedIn) {
		return s.db.PhotoLikes(photo)
	} else {
		return nil, NotFoundError("photo not found")
	}
}

func (s *mserver) handleVerifyGuest(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	type request struct {
		Code string
	}
	var params request
	var err error
	var id uuid.UUID
	if err = decodeRequest(r, &params); err != nil {
		return nil, err
	}
	if id, err = uuid.Parse(params.Code); err != nil {
		return nil, BadRequestError("could not parse verification code: " + err.Error())
	}
	if ver, err := s.db.VerifyGuest(id); err != nil {
		return nil, err
	} else if ver.Verified {
		return ver, s.saveGuestCookie(w, r, id, Session_Year)
	} else {
		return ver, nil
	}
}

func (s *mserver) handleIsGuest(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var guest = ctxGuest(r.Context())
	if guest == emptyuuid {
		return AuthUser{false}, nil
	} else {
		return AuthUser{true}, nil
	}
}

func (s *mserver) handleGuestLikes(r *http.Request, uuid uuid.UUID) (interface{}, error) {
	if likes, err := s.db.GuestLikes(uuid); err != nil {
		return nil, err
	} else {
		return likes, nil
	}
	return nil, nil
}

func (s *mserver) handleLogoutGuest(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return AuthUser{false}, s.saveGuestCookie(w, r, emptyuuid, -1)
}

func (s *mserver) handleGuestLikePhoto(r *http.Request, uuid uuid.UUID) (interface{}, error) {
	type ret struct {
		Like bool `json:"like"`
	}
	photo := Var(r, "photo")
	like := s.db.Like(uuid, photo)
	return ret{like}, nil
}

func (s *mserver) handleCreateGuest(w http.ResponseWriter, r *http.Request) (interface{}, error) {

	sendEmail := func(g *model.Guest, uuid *uuid.UUID) error {
		var b strings.Builder
		we := WelcomeEmail{Name: g.Name, Code: uuid.String(), VerifyUrl: config.VerifyUrl()}
		if err := templates.ExecuteTemplate(&b, "welcome-email.html", we); err != nil {
			return err
		}
		_, err := s.ms.SendHtmlMessage(g.Email, "Mellowtech Guest Verification", b.String())
		return err
	}

	type request struct {
		Email string
		Name  string
	}

	var params request

	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	if s.db.HasGuestByEmail(params.Email) {
		s.l.Debugw("guest already exists send a new verify email", "email", params.Email)
		id, _ := s.db.GuestUUID(params.Email)
		u, _ := s.db.Guest(*id)

		if u.Name != params.Name {
			return nil, UnauthorizedError("name does not match provided email")
		}
		//here we should send a notification email...not a verify email
		if err := sendEmail(u, id); err != nil {
			return nil, err
		}
		return u, s.saveGuestCookie(w, r, *id, Session_Year)
	}
	//user email not found try to create new user
	id := uuid.New()
	u := model.Guest{
		Email: params.Email,
		Name:  params.Name,
	}
	if s.db.HasGuestByName(params.Name) {
		return nil, UnauthorizedError("name already exists")
	}
	if err := s.db.AddGuest(id, &u); err != nil {
		return nil, err
	}
	if err := sendEmail(&u, &id); err != nil {
		_, _ = s.db.DeleteGuest(id)
		return nil, err
	}
	return u, s.saveGuestCookie(w, r, id, Session_Year)
}

func (s *mserver) handleUpdateGuest(r *http.Request, uuid uuid.UUID) (interface{}, error) {

	type request struct {
		Email string
		Name  string
	}

	var params request

	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}

	if strings.TrimSpace(params.Name) == "" {
		return nil, BadRequestError("name cannot contain only white space characters")
	}
	u, _ := s.db.Guest(uuid)

	if params.Email != "" && params.Email != u.Email {
		return nil, BadRequestError("change of email is not yet supported")
	}
	return s.db.UpdateGuest(uuid, params.Email, params.Name)

}
