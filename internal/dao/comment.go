package dao

import (
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"time"
)

type CommentPG struct {
	db                *sqlx.DB
	commentFields     []string
	insertCommentStmt string
}

func NewCommentPG(db *sqlx.DB) *CommentPG {
	c := &Comment{}
	fields := getStructFields(c)
	stmt := buildInsertNamed("comment", fields, "id") + " RETURNING ID"
	return &CommentPG{db, fields, stmt}
}

func (dao *CommentPG) Add(guestId uuid.UUID, photoId uuid.UUID, body string) (*Comment, error) {
	c := Comment{GuestId: guestId, PhotoId: photoId, Body: body, Time: time.Now()}
	if rows, err := dao.db.NamedQuery(dao.insertCommentStmt, &c); err != nil {
		return nil, err
	} else {
		rows.Next()
		var id int
		err = rows.Scan(&id)
		c.Id = id
		return &c, err
	}
}

func (dao *CommentPG) Get(id int) (*Comment, error) {
	ret := Comment{}
	err := dao.db.Get(&ret, "SELECT * FROM comment WHERE id = $1")
	return &ret, err
}

func (dao *CommentPG) Delete(id int) error {
	_, err := dao.db.Exec("DELETE from comment WHERE id = $1", id)
	return err
}

func (dao *CommentPG) DeleteByPhoto(photoId uuid.UUID) error {
	_, err := dao.db.Exec("DELETE from comment WHERE photoId = $1", photoId)
	return err
}

func (dao *CommentPG) DeleteByGuest(guestId uuid.UUID) error {
	_, err := dao.db.Exec("DELETE from comment WHERE guestId = $1", guestId)
	return err
}

func (dao *CommentPG) List() ([]*Comment, error) {
	ret := []*Comment{}
	err := dao.db.Select(&ret, "SELECT * FROM comment ORDER BY time DESC")
	return ret, err
}

func (dao *CommentPG) ListByPhoto(photoId uuid.UUID) ([]*Comment, error) {
	ret := []*Comment{}
	err := dao.db.Select(&ret, "SELECT * FROM comment WHERE photoid = $1 ORDER BY time DESC", photoId)
	return ret, err
}

func (dao *CommentPG) ListByGuest(guestId uuid.UUID) ([]*Comment, error) {
	ret := []*Comment{}
	err := dao.db.Select(&ret, "SELECT * FROM comment WHERE guestid = $1 ORDER BY time DESC", guestId)
	return ret, err
}
