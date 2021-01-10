package model

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

type Comment struct {
	Id      int       `json:"id"`
	Guest   string    `json:"guest"`
	driveId string    `json:"driveId"`
	Time    time.Time `json:"time"`
	Body    string    `json:"body"`
}

type CommentStore interface {
	CreateCommentStore() error
	DeleteCommentStore() error
	AddComment(guestId uuid.UUID, driveId string, body string) (*Comment, error)
	PhotoComments(driveId string) ([]*Comment, error)
}

func (db *DB) CreateCommentStore() error {
	const stmt = `
CREATE TABLE IF NOT EXISTS comments (
	id SERIAL PRIMARY KEY,
	guestId UUID NOT NULL,
	driveId TEXT,
	ts TIMESTAMP NOT NULL,
	body TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS driveId_idx ON comments (driveId);
`
	_, err := db.Exec(stmt)
	fmt.Println("create comment store, ", err)
	return err
}

func (db *DB) DeleteCommentStore() error {
	_, err := db.Exec("DROP TABLE IF EXISTS guests;")
	return err
}

func (db *DB) AddComment(guestId uuid.UUID, driveId string, body string) (*Comment, error) {
	const stmt = "INSERT INTO comments (guestId, driveId, ts, body) VALUES ($1, $2, $3, $4) RETURNING id"
	ts := time.Now()
	var id int
	err := db.QueryRow(stmt, guestId, driveId, ts, body).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &Comment{id, "", driveId, ts, body}, nil
}

func (db *DB) PhotoComments(driveId string) ([]*Comment, error) {
	//const stmt = "SELECT id, guestId, driveId, ts, body FROM comments WHERE driveId = $1"
	const stmt = `
select comments.id, name, driveId, ts, body FROM comments, guests WHERE driveId = $1 AND comments.guestid = guests.id
`
	comments := []*Comment{}
	if rows, err := db.Query(stmt, driveId); err != nil {
		return nil, err
	} else {
		defer rows.Close()
		for rows.Next() {
			var comment = Comment{}
			if err := rows.Scan(&comment.Id, &comment.Guest, &comment.driveId, &comment.Time, &comment.Body); err != nil {
				return nil, err
			}
			comments = append(comments, &comment)
		}
	}
	return comments, nil
}
