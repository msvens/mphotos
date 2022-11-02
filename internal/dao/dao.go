package dao

import (
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/msvens/mimage/metadata"

	//"github.com/msvens/mexif"
	"github.com/msvens/mphotos/internal/config"
	"go.uber.org/zap"
)

type AlbumDAO interface {
	Add(name, description, coverpic string) (*Album, error)
	Get(id uuid.UUID) (*Album, error)
	List() ([]*Album, error)
	Photos(id uuid.UUID, private bool) ([]*Photo, error)
	Delete(id uuid.UUID) error
	Has(id uuid.UUID) bool
	HasByName(name string) bool
	Albums(photoId uuid.UUID) ([]*Album, error)
	Update(album *Album) (*Album, error)
	UpdatePhoto(albumIds []uuid.UUID, photoId uuid.UUID) error
}

type CameraDAO interface {
	Add(camera *Camera) error
	AddFromPhoto(photo *Photo) error
	AddFromPhotos(photos []*Photo) error
	Get(id string) (*Camera, error)
	List() ([]*Camera, error)
	Delete(id string) error
	Has(id string) bool
	HasModel(model string) bool
	Update(camera *Camera) (*Camera, error)
	UpdateImage(img, id string) (*Camera, error)
}

type CommentDAO interface {
	Add(guestId uuid.UUID, photoId uuid.UUID, body string) (*Comment, error)
	Get(id int) (*Comment, error)
	Delete(id int) error
	DeleteByPhoto(photoId uuid.UUID) error
	DeleteByGuest(guestId uuid.UUID) error
	List() ([]*Comment, error)
	ListByPhoto(photoId uuid.UUID) ([]*Comment, error)
	ListByGuest(photoId uuid.UUID) ([]*Comment, error)
}

type GuestDAO interface {
	Add(name, email string) (*Guest, error)
	Delete(id uuid.UUID) error
	Verify(id uuid.UUID) (*Guest, error)
	Get(id uuid.UUID) (*Guest, error)
	GetByEmail(email string) (*Guest, error)
	Has(id uuid.UUID) bool
	HasByEmail(email string) bool
	HasByName(name string) bool
	Update(email string, name string, id uuid.UUID) (*Guest, error)
}

type ReactionDAO interface {
	Add(reaction *Reaction) error
	Delete(reaction *Reaction) error
	DeleteByGuest(guest uuid.UUID) error
	DeleteByPhoto(photoId uuid.UUID) error
	List() ([]*Reaction, error)
	ListByGuest(guestId uuid.UUID) ([]uuid.UUID, error)
	ListByPhoto(photoId uuid.UUID) ([]*GuestReaction, error)
	Has(guest uuid.UUID, photoId uuid.UUID) bool
}

type PhotoDAO interface {
	Add(p *Photo, exif *metadata.Summary) error
	Delete(id uuid.UUID) (bool, error)
	Exif(id uuid.UUID) (*Exif, error)
	Has(id uuid.UUID, private bool) bool
	HasMd5(md5 string) bool
	Get(id uuid.UUID, private bool) (*Photo, error)
	List() ([]*Photo, error)
	ListSource(source string) ([]*Photo, error)
	Select(r Range, order PhotoOrder, filter PhotoFilter) ([]*Photo, error)
	SetPrivate(private bool, id uuid.UUID) (*Photo, error)
	Set(title string, description string, keywords []string, id uuid.UUID) (*Photo, error)
}

type UserDAO interface {
	Update(u *User) (*User, error)
	Get() (*User, error)
}

type VersionDAO interface {
	Get() (*Version, error)
	Update() (*Version, error)
	IsCurrent() (bool, error)
}

type PGDB struct {
	db       *sqlx.DB
	Album    AlbumDAO
	Camera   CameraDAO
	Comment  CommentDAO
	Guest    GuestDAO
	Photo    PhotoDAO
	Reaction ReactionDAO
	User     UserDAO
	Version  VersionDAO
}

var logger *zap.SugaredLogger

//var pg *PGDB = nil

func init() {
	l, _ := zap.NewDevelopment()
	logger = l.Sugar()
}

func NewPGDB() (*PGDB, error) {
	dataSource := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.DbHost(), config.DbPort(), config.DbUser(), config.DbPassword(), config.DbName())
	if db, err := sqlx.Open("pgx", dataSource); err != nil {
		logger.Errorw("could not connect to database", zap.Error(err))
		return nil, err
	} else {
		err = db.Ping()
		if err != nil {
			logger.Errorw("could not ping database", zap.Error(err))
			return nil, err
		}
		return &PGDB{
			db:       db,
			Album:    NewAlbumPG(db),
			Camera:   NewCameraPG(db),
			Comment:  NewCommentPG(db),
			Guest:    NewGuestPG(db),
			Photo:    NewPhotoPG(db),
			Reaction: NewReactionPG(db),
			User:     NewUserPG(db),
			Version:  NewVersionPG(db),
		}, nil
	}
}

func (pgd *PGDB) Close() error {
	err := pgd.db.Close()
	return err
}

func (pgd *PGDB) tableExists(table string) bool {
	var rel sql.NullString
	q := fmt.Sprintf("SELECT to_regclass('public.%s')", table)
	row := pgd.db.QueryRow(q)
	if err := row.Scan(&rel); err != nil {
		return false
	} else {
		return rel.Valid
	}

}

func (pgd *PGDB) CreateTables() error {
	//pgd.db.MustExec(schemaV1)
	_, err := pgd.db.Exec(schemaV1)
	return err
}

func (pgd *PGDB) DeleteTables() error {
	_, err := pgd.db.Exec(deleteSchemaV1)
	return err
}
