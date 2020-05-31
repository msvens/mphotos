package service

import (
	"github.com/msvens/mdrive"
	"github.com/msvens/mexif"
	"github.com/msvens/mphotos/config"
	"go.uber.org/zap"
	"google.golang.org/api/drive/v3"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var logger *zap.SugaredLogger

const (
	folderFileName = "gdriveFolder.json"
	fileFields     = "id, name, kind, mimeType, md5Checksum, createdTime"
)

type PhotoService struct {
	DriveSrv *mdrive.DriveService
	//driveFolder *drive.File
	rootDir    string
	imgDir     string
	thumbDir   string
	folderPath string
	//photoList *PhotoList
	dbs *DbService
}

func NewPhotosService(driveSrv *mdrive.DriveService) (*PhotoService, error) {
	l, _ := zap.NewDevelopment()
	logger = l.Sugar()
	//srvPath = config.ServiceRoot()
	ps := PhotoService{}
	ps.rootDir = config.ServiceRoot()
	ps.imgDir = config.ServicePath("img")
	ps.thumbDir = config.ServicePath("thumb")
	if err := ps.createPaths(); err != nil {
		logger.Errorw("could not create image folders", "error", err)
		return nil, err
	}
	ps.DriveSrv = driveSrv
	ps.folderPath = ps.rootDir + "/" + folderFileName

	//Open db
	var err error
	if ps.dbs, err = NewDbService(); err != nil {
		logger.Errorw("could not create dbservice", "error", err)
		return nil, err
	} else {
		if err = ps.dbs.CreateTables(); err != nil {
			logger.Errorw("could not create db talbes", "error", err)
			return nil, err
		}

	}
	wg.Add(1)
	go worker(jobChan)
	logger.Info("PhotoService started")
	return &ps, nil
}

func (ps *PhotoService) Shutdown() error {
	close(jobChan)
	wg.Wait()
	logger.Sync()
	return nil
}

func (ps *PhotoService) createPaths() error {
	var err error
	if err = os.MkdirAll(ps.imgDir, 0744); err != nil {
		return err
	}
	if err = os.MkdirAll(ps.thumbDir, 0744); err != nil {
		return err
	}
	return nil
}

func (ps *PhotoService) GetExif(id string) (*Exif, bool) {
	if exif, err := ps.dbs.GetExif(id); err == nil {
		return exif, true
	} else {
		logger.Errorw("could get exif", "error", err)
		return nil, false
	}
}

func (ps *PhotoService) GetPhoto(id string) (*Photo, bool) {
	if p, err := ps.dbs.GetId(id); err == nil {
		return p, true
	} else {
		logger.Errorw("could not get photo", "error", err)
		return nil, false
	}
}

func (ps *PhotoService) GetLatestPhoto() (*Photo, bool) {
	if p, err := ps.dbs.GetLatest(); err == nil {
		return p, true
	} else {
		logger.Info(err)
		return nil, false
	}
}

func (ps *PhotoService) GetPhotos(originalDate bool, limit int, offset int) (*PhotoFiles, error) {
	var err error
	var photos []*Photo
	if originalDate {
		photos, err = ps.dbs.GetByOriginalDate(limit, offset)
	} else {
		photos, err = ps.dbs.GetByDriveDate(limit, offset)
	}
	if err != nil {
		return nil, err
	}
	return &PhotoFiles{Length: len(photos), Photos: photos}, nil
}

func (ps *PhotoService) GetUser() (*User, error) {
	return ps.dbs.GetUser()
}

func (ps *PhotoService) UpdatePhoto(driveId string, title string, description string, keywords []string) (*Photo, error) {
	return ps.dbs.UpdatePhoto(title, description, keywords, driveId)
}

func (ps *PhotoService) UpdateUserDrive(name string) (*User, error) {
	if name == "" {
		return nil, BadRequestError("name is empty")
	}
	if f, err := ps.DriveSrv.GetByName(name, true, false, fileFields); err != nil {
		return nil, err
	} else {
		return ps.dbs.UpdateUserDriveFolder(f.Id, f.Name)
	}
}

func (ps *PhotoService) UpdateUserPic(picUrl string) (*User, error) {
	return ps.dbs.UpdateUserPic(picUrl)
}

func (ps *PhotoService) UpdateUser(user *User) (*User, error) {
	return ps.dbs.UpdateUser(user)
}

func (ps *PhotoService) GetImgPath(fileName string) string {
	return filepath.Join(ps.imgDir, fileName)
}

func (ps *PhotoService) GetThumbPath(fileName string) string {
	return filepath.Join(ps.thumbDir, fileName)
}

func (ps *PhotoService) SearchDrive(id string, name string) (*DriveFiles, error) {
	if files, err := ps.SearchDriveFiles(id, name); err != nil {
		return nil, err
	} else {
		return ToDriveFiles(files), nil
	}
}

func (ps *PhotoService) SearchDriveFiles(id string, name string) ([]*drive.File, error) {
	if name != "" {
		if f, err := ps.DriveSrv.GetByName(name, true, false, fileFields); err != nil {
			return nil, err
		} else {
			id = f.Id
		}
	}
	query := mdrive.NewQuery().Parents().In(id).And().MimeType().Eq(mdrive.Jpeg).TrashedEq(false)
	return ps.DriveSrv.SearchAll(query, fileFields)
}

func (ps *PhotoService) ListDrive() (*DriveFiles, error) {
	if files, err := ps.ListDriveFiles(); err != nil {
		return nil, err
	} else {
		return ToDriveFiles(files), nil
	}
}

func (ps *PhotoService) ListDriveFiles() ([]*drive.File, error) {
	if u, err := ps.GetUser(); err != nil {
		return nil, InternalError("user not found")
	} else if u.DriveFolderId == "" {
		return nil, NotFoundError("Drive folder has not been set")
	} else {
		return ps.SearchDriveFiles(u.DriveFolderId, "")
	}
}

func (ps *PhotoService) CheckPhotos() (*DriveFiles, error) {
	if files, err := ps.CheckPhotosDrive(); err != nil {
		return nil, err
	} else {
		return ToDriveFiles(files), nil
	}
}

func (ps *PhotoService) CheckPhotosDrive() ([]*drive.File, error) {
	fl, err := ps.ListDriveFiles()
	if err != nil {
		return nil, err
	}
	var ret []*drive.File
	for _, f := range fl {
		if !ps.dbs.Contains(f.Id) {
			ret = append(ret, f)
		}
	}
	return ret, nil
}

func (ps *PhotoService) AddPhotos() (*DriveFiles, error) {
	fl, err := ps.ListDriveFiles()
	if err != nil {
		return nil, err
	}

	tool, err := mexif.NewMExifTool()
	if err != nil {
		return nil, err
	}

	defer tool.Close()

	var files []*drive.File
	for _, f := range fl {
		added, err := ps.AddPhoto(f, tool)
		if err != nil {
			return nil, err
		}
		if added {
			files = append(files, f)
		}
	}
	return ToDriveFiles(files), nil
}

func (ps *PhotoService) AddPhoto(f *drive.File, tool *mexif.MExifTool) (bool, error) {
	var err error
	if ps.dbs.Contains(f.Id) {
		return false, nil
	}
	photo := Photo{}
	photo.DriveId = f.Id
	//photo.Title = f.Name
	photo.Md5 = f.Md5Checksum
	photo.FileName = f.Id + ".jpg"
	if t, err := mdrive.ParseTime(f.CreatedTime); err == nil {
		photo.DriveDate = t
	}

	if err = ps.downloadPhoto(&photo); err != nil {
		logger.Errorw("error downloading photo", zap.Error(err))
		return false, err
	}
	var exif *mexif.ExifCompact

	if exif, err = tool.ExifCompact(ps.GetImgPath(photo.FileName)); err == nil {
		photo.CameraMake = exif.CameraMake
		photo.CameraModel = exif.CameraModel
		photo.FocalLength = exif.FocalLength
		photo.FocalLength35 = exif.FocalLengthIn35mmFormat
		photo.LensMake = exif.LensMake
		photo.LensModel = exif.LensModel
		photo.Exposure = exif.ExposureTime
		photo.Width = exif.ImageWidth
		photo.Height = exif.ImageHeight
		photo.FNumber = exif.FNumber
		photo.Iso = exif.ISO
		photo.Title = exif.Title
		if len(exif.Keywords) > 0 {
			photo.Keywords = strings.Join(exif.Keywords, ",")
		}
		photo.OriginalDate = exif.OriginalDate
	} else {
		return false, err
	}

	if err = ps.dbs.AddPhoto(&photo, exif); err != nil {
		logger.Errorw("error adding photo: ", zap.Error(err))
		return false, err
	}
	logger.Infow("added photo", "driveId", photo.DriveId)
	return true, nil
}

func (ps *PhotoService) DeletePhotos(removeFiles bool) (*PhotoFiles, error) {
	logger.Infow("Delete All Photos", "removeFiles", removeFiles)
	if photos, err := ps.dbs.GetAllPhotos(); err != nil {
		return nil, err
	} else {
		for _, p := range photos {
			if _, e := ps.DeletePhoto(p, removeFiles); e != nil {
				return nil, e
			}
		}
		return &PhotoFiles{Length: len(photos), Photos: photos}, nil
	}
}

func (ps *PhotoService) DeletePhoto(p *Photo, removeFiles bool) (*Photo, error) {
	logger.Infow("Delete Photo", "id", p.DriveId, "removeFiles", removeFiles)
	if del, err := ps.dbs.Delete(p.DriveId); err != nil {
		return nil, err
	} else if !del {
		return nil, NotFoundError("Photo not found")
	}
	if !removeFiles {
		return p, nil
	}
	//remove files
	if err := os.Remove(ps.GetImgPath(p.FileName)); err != nil {
		logger.Errorw("Could not remove image", zap.Error(err))
	}
	if err := os.Remove(ps.GetThumbPath(p.FileName)); err != nil {
		logger.Errorw("Could not remove thumbnail", zap.Error(err))
	}
	logger.Infow("Photo deleted", "id", p.DriveId)
	return p, nil

}

func (ps *PhotoService) downloadPhoto(photo *Photo) error {

	if _, err := ps.DriveSrv.Download(photo.DriveId, ps.GetImgPath(photo.FileName)); err != nil {
		return err
	}

	//create thumbnail
	//args := []string{ps.GetImgPath(photo.FileName), "-s", "640", "-m", "centre", "-o", ps.GetThumbPath(photo.FileName)}
	args := []string{ps.GetImgPath(photo.FileName), "-s", "640", "-c", "-o", ps.GetThumbPath(photo.FileName)}
	logger.Infow("creating thumbnail", "args: ", strings.Join(args, " "))
	cmd := exec.Command("vipsthumbnail", args...)

	if err := cmd.Start(); err != nil {
		logger.Errorw("could not create thumbnail", zap.Error(err))
		return InternalError(err.Error())
	}

	return nil
}
