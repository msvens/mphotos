package server

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"html/template"
	"net/http"
	"strings"
	"time"
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
		if s.pg.Guest.Has(uuid) {
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
	fmt.Println("this is guest id: ", gid.Id)
	session.Values["guest"] = gid
	session.Options.MaxAge = days
	if err := session.Save(r, w); err != nil {
		return InternalError(err.Error())
	}
	return nil
}

func (s *mserver) handleCommentPhoto(r *http.Request, uid uuid.UUID) (interface{}, error) {
	if photoId, err := uuid.Parse(Var(r, "photo")); err != nil {
		return nil, BadRequestError("Could not parse photo id")
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
	if photoId, err := uuid.Parse(Var(r, "photo")); err != nil {
		return nil, BadRequestError("Could not parse photo id")
	} else {
		type resp struct {
			Id      int       `json:"id"`
			Name    string    `json:"name"`
			PhotoId uuid.UUID `json:"photoId"`
			Time    time.Time `json:"time"`
			Body    string    `json:"body"`
		}
		comments, err := s.pg.Comment.ListByPhoto(photoId)
		if err != nil {
			return nil, err
		}
		ret := []*resp{}
		for _, c := range comments {
			u, _ := s.pg.Guest.Get(c.GuestId)
			ret = append(ret, &resp{Id: c.Id, Name: u.Name, PhotoId: c.PhotoId, Time: c.Time, Body: c.Body})
		}
		return ret, nil
	}
}

func (s *mserver) handleGuest(r *http.Request, uuid uuid.UUID) (interface{}, error) {
	return s.pg.Guest.Get(uuid)
}

func (s *mserver) handleLikePhoto(r *http.Request, guestId uuid.UUID) (interface{}, error) {
	if photoId, err := uuid.Parse(Var(r, "photo")); err != nil {
		return nil, BadRequestError("Could not parse photo id")
	} else {
		if s.pg.Photo.Has(photoId, false) {
			return photoId, s.pg.Reaction.Add(&dao.Reaction{GuestId: guestId, PhotoId: photoId, Kind: "like"})
		} else {
			return nil, NotFoundError("photo not found")
		}
	}
}

func (s *mserver) handleUnlikePhoto(r *http.Request, guestId uuid.UUID) (interface{}, error) {
	var photoId uuid.UUID
	if err := uid(r, "photo", &photoId); err != nil {
		return nil, err
	}
	if s.pg.Photo.Has(photoId, false) {
		return photoId, s.pg.Reaction.Delete(&dao.Reaction{GuestId: guestId, PhotoId: photoId, Kind: "like"})
	} else {
		return nil, NotFoundError("photo not found")
	}
}

func (s *mserver) handlePhotoLikes(r *http.Request, loggedIn bool) (interface{}, error) {
	if photoId, err := uuid.Parse(Var(r, "photo")); err != nil {
		return nil, BadRequestError("Could not parse photo id")
	} else {
		return s.pg.Reaction.ListByPhoto(photoId)
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
		return nil, BadRequestError("could not parse guest id: " + err.Error())
	}
	if ver, err := s.pg.Guest.Verify(id); err != nil {
		return nil, err
	} else if ver.Verified {
		return ver, s.saveGuestCookie(w, r, id, Session_Year)
	} else {
		return ver, nil
	}
}

func (s *mserver) handleIsGuest(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var guest = ctxGuest(r.Context())
	fmt.Println("Guest Id: ", guest)
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
	if photoId, err := uuid.Parse(Var(r, "photo")); err != nil {
		return nil, BadRequestError("Could not parse photo id")
	} else {
		type ret struct {
			Like bool `json:"like"`
		}
		return ret{s.pg.Reaction.Has(guestId, photoId)}, nil
	}
}

func (s *mserver) handleCreateGuest(w http.ResponseWriter, r *http.Request) (interface{}, error) {

	sendEmail := func(g *dao.Guest) error {
		var b strings.Builder
		we := WelcomeEmail{Name: g.Name, Code: g.Id.String(), VerifyUrl: config.VerifyUrl()}
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
	if s.pg.Guest.HasByEmail(params.Email) {
		s.l.Debugw("guest already exists send a new verify email", "email", params.Email)
		guest, _ := s.pg.Guest.GetByEmail(params.Email)

		if guest.Name != params.Name {
			return nil, UnauthorizedError("name does not match provided email")
		}
		//here we should send a notification email...not a verify email
		if err := sendEmail(guest); err != nil {
			return nil, err
		}
		return guest, s.saveGuestCookie(w, r, guest.Id, Session_Year)
	}
	//user email not found try to create new user
	if s.pg.Guest.HasByName(params.Name) {
		return nil, UnauthorizedError("name already exists")
	}
	//add new guest
	if guest, err := s.pg.Guest.Add(params.Name, params.Email); err != nil {
		return nil, err
	} else {
		if err := sendEmail(guest); err != nil {
			_ = s.pg.Guest.Delete(guest.Id)
			return nil, err
		}
		return guest, s.saveGuestCookie(w, r, guest.Id, Session_Year)
	}

}

func (s *mserver) handleUpdateGuest(r *http.Request, guestId uuid.UUID) (interface{}, error) {

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
	guest, _ := s.pg.Guest.Get(guestId)

	if params.Email != "" && params.Email != guest.Email {
		return nil, BadRequestError("change of email is not yet supported")
	}
	return s.pg.Guest.Update(params.Email, params.Name, guest.Id)
}
