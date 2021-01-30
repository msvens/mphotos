package model

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/msvens/mphotos/internal/config"
	"go.uber.org/zap"
	"strings"
)

type DataStore interface {
	PhotoStore
	AlbumStore
	UserStore
	GuestStore
	CommentStore
	LikeStore
	CreateDataStore() error
	DeleteDataStore() error
	CloseDb() error
}

type DB struct {
	*sql.DB
}

var logger *zap.SugaredLogger

func init() {
	l, _ := zap.NewDevelopment()
	logger = l.Sugar()
}

func NewDB() (DataStore, error) {

	dataSource := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.DbHost(), config.DbPort(), config.DbUser(), config.DbPassword(), config.DbName())
	if db, err := sql.Open("postgres", dataSource); err != nil {
		logger.Errorw("could not connect to database", zap.Error(err))
		return nil, err
	} else {
		err = db.Ping()
		if err != nil {
			logger.Errorw("could not ping database", zap.Error(err))
			return nil, err
		}
		return &DB{db}, nil
	}
}

func (db *DB) CloseDb() error {
	return db.Close()
}

func (db *DB) CreateDataStore() error {
	var err error
	if err = db.CreateUserStore(); err != nil {
		return err
	}
	if err = db.CreateAlbumStore(); err != nil {
		return err
	}
	if err = db.CreateGuestStore(); err != nil {
		return err
	}
	if err = db.CreateLikeStore(); err != nil {
		return err
	}
	if err = db.CreateCommentStore(); err != nil {
		return err
	}
	return db.CreatePhotoStore()
}

func (db *DB) DeleteDataStore() error {
	var err error
	if err = db.DeleteUserStore(); err != nil {
		return err
	}
	if err = db.DeleteAlbumStore(); err != nil {
		return err
	}
	if err = db.DeleteGuestStore(); err != nil {
		return err
	}
	if err = db.DeleteLikeStore(); err != nil {
		return err
	}
	if err = db.DeleteCommentStore(); err != nil {
		return err
	}
	return db.DeletePhotoStore()
}

func trimAndJoin(strs []string) string {
	var newString []string
	for _, str := range strs {
		newString = append(newString, strings.TrimSpace(str))
	}
	return strings.Join(newString, ",")
}

func trimAndSplit(str string) []string {
	strs := split(str)
	var ret []string
	for _, s := range strs {
		ret = append(ret, strings.TrimSpace(s))
	}
	return ret
}

func split(str string) []string {
	return strings.Split(str, ",")
}

func trim(str string) string {
	return strings.TrimSpace(str)
}
