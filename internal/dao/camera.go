package dao

import (
	"github.com/jmoiron/sqlx"
	"regexp"
	"strings"
)

type CameraPG struct {
	db           *sqlx.DB
	cameraFields []string
	insertStmt   string
	updateStmt   string
}

func NewCameraPG(db *sqlx.DB) *CameraPG {
	c := &Camera{}
	fields := getStructFields(c)
	return &CameraPG{db, fields,
		buildInsertNamed("camera", fields),
		buildUpdateNamed2("camera", fields, "id", "id")}
}

var space = regexp.MustCompile(`\s+`)

func convertModel(model string) string {
	return space.ReplaceAllString(strings.ToLower(model), "-")
}

func (dao *CameraPG) Add(c *Camera) error {
	c.Id = convertModel(c.Model)
	if _, err := dao.db.NamedExec(dao.insertStmt, c); err != nil {
		return err
	}
	return nil
}

func (dao *CameraPG) AddFromPhoto(p *Photo) error {
	return dao.Add(&Camera{Model: p.CameraModel, Make: p.CameraMake})
}

func (dao *CameraPG) AddFromPhotos(photos []*Photo) error {
	for _, p := range photos {
		if err := dao.AddFromPhoto(p); err != nil {
			return err
		}

	}
	return nil
}

func (dao *CameraPG) Get(id string) (*Camera, error) {
	ret := Camera{}
	err := dao.db.Get(&ret, "SELECT * from camera WHERE id = $1", id)
	return &ret, err
}

func (dao *CameraPG) List() ([]*Camera, error) {
	ret := []*Camera{}
	err := dao.db.Select(&ret, "SELECT * FROM camera")
	return ret, err
}
func (dao *CameraPG) Delete(id string) error {
	_, err := dao.db.Exec("DELETE FROM camera WHERE id = $1", id)
	return err
}

func (dao *CameraPG) Has(id string) bool {
	return has(dao.db, "camera", "id", id)
	/*if rows, err := dao.db.Query("SELECT 1 FROM camera WHERE id = $1", id); err == nil {
		defer rows.Close()
		return rows.Next()
	} else {
		return false
	}*/
}

func (dao *CameraPG) HasModel(model string) bool {
	return dao.Has(convertModel(model))
}

func (dao *CameraPG) Update(camera *Camera) (*Camera, error) {
	if _, err := dao.db.NamedExec(dao.updateStmt, camera); err != nil {
		return nil, err
	}
	return dao.Get(camera.Id)

}

func (dao *CameraPG) UpdateImage(img, id string) (*Camera, error) {
	if _, err := dao.db.Exec("UPDATE cameras SET image = $1 WHERE id = $2", img, id); err != nil {
		return nil, err
	}
	return dao.Get(id)
}
