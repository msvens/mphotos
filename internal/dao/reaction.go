package dao

import (
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type ReactionPG struct {
	db              *sqlx.DB
	guestFields     []string
	addReactionStmt string
}

func NewReactionPG(db *sqlx.DB) *ReactionPG {
	fields := getStructFields(&Reaction{})
	stmt := buildInsertNamed("reaction", fields)
	return &ReactionPG{db, fields, stmt}
}

func (dao *ReactionPG) Add(r *Reaction) error {
	_, err := dao.db.NamedExec(dao.addReactionStmt, r)
	return err
}

func (dao *ReactionPG) Delete(r *Reaction) error {
	_, err := dao.db.Exec("DELETE from reaction WHERE guestId = $1 AND photoId = $2", r.GuestId, r.PhotoId)
	return err
}

func (dao *ReactionPG) DeleteByGuest(guestId uuid.UUID) error {
	_, err := dao.db.Exec("DELETE from reaction WHERE guestId = $1", guestId)
	return err
}

func (dao *ReactionPG) DeleteByPhoto(photoId uuid.UUID) error {
	_, err := dao.db.Exec("DELETE from reaction WHERE photoId = $1", photoId)
	return err
}
func (dao *ReactionPG) List() ([]*Reaction, error) {
	ret := []*Reaction{}
	err := dao.db.Select(&ret, "SELECT * FROM reaction")
	return ret, err
}

func (dao *ReactionPG) ListByGuest(guestId uuid.UUID) ([]uuid.UUID, error) {
	ret := []uuid.UUID{}
	err := dao.db.Select(&ret, "SELECT photoId FROM reaction WHERE guestId = $1", guestId)
	return ret, err
}

func (dao *ReactionPG) ListByPhoto(photoId uuid.UUID) ([]*GuestReaction, error) {
	const stmt = "select name,email,kind FROM reaction, guest WHERE photoId = $1 AND reaction.guestId = guest.id"
	ret := []*GuestReaction{}
	err := dao.db.Select(&ret, stmt, photoId)
	return ret, err
}

func (dao *ReactionPG) Has(guestId uuid.UUID, photoId uuid.UUID) bool {
	if rows, err := dao.db.Query("SELECT 1 FROM reaction WHERE guestId = $1 AND photoId = $2", guestId, photoId); err == nil {
		defer rows.Close()
		return rows.Next()
	} else {
		return false
	}

}
