package server

import (
	"github.com/google/uuid"
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
	Session_Year int = 60 * 60 * 24 * 365
)

var emptyuuid = uuid.UUID{}

type WelcomeEmail struct {
	VerifyUrl string
	Code      string
	Name      string
}

var templates = template.Must(template.ParseFiles("tmpl/welcome-email.html"))

func guestUUID(w http.ResponseWriter, r *http.Request, s *mserver) (uuid.UUID, error) {
	session, err := s.store.Get(r, s.guestCookie)
	if err != nil {
		return emptyuuid, InternalError(err.Error())
	}
	val := session.Values["guest"]
	var guest = SessionGuest{}
	guest, ok := val.(SessionGuest)
	if !ok {
		return emptyuuid, NotFoundError("no guest cookie")
	}
	//check guest
	if uuid, err := uuid.Parse(guest.Id); err != nil {
		return uuid, err
	} else {
		if s.db.HasGuest(uuid) {
			return uuid, nil
		} else {
			//set the max age to -1 as the uuid is invalid
			session.Values["guest"] = SessionGuest{}
			session.Options.MaxAge = -1
			if err := session.Save(r, w); err != nil {
				return emptyuuid, InternalError(err.Error())
			}
			return emptyuuid, UnauthorizedError("guest not found")
		}
	}
}

func (s *mserver) saveGuestCookie(w http.ResponseWriter, r *http.Request, guest uuid.UUID) error {
	session, err := s.store.Get(r, s.guestCookie)
	if err != nil {
		return InternalError(err.Error())
	}
	gid := &SessionGuest{guest.String()}
	session.Values["guest"] = gid
	session.Options.MaxAge = Session_Year
	if err := session.Save(r, w); err != nil {
		return InternalError(err.Error())
	}
	return nil
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
		return ver, s.saveGuestCookie(w, r, id)
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

	saveGuestCookie := func(id *uuid.UUID) error {
		session, err := s.store.Get(r, s.guestCookie)
		if err != nil {
			return InternalError(err.Error())
		}
		gid := &SessionGuest{id.String()}
		session.Values["guest"] = gid
		session.Options.MaxAge = Session_Year
		if err := session.Save(r, w); err != nil {
			return InternalError(err.Error())
		}
		return nil
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
		if err := sendEmail(u, id); err != nil {
			return nil, err
		}
		return u, saveGuestCookie(id)
	}
	//user not found
	id := uuid.New()
	u := model.Guest{
		Email: params.Email,
		Name:  params.Name,
	}
	if err := s.db.AddGuest(id, &u); err != nil {
		return nil, err
	}
	if err := sendEmail(&u, &id); err != nil {
		return nil, err
	}
	return u, saveGuestCookie(&id)
}
