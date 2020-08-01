package service

import (
	"github.com/msvens/mdrive"
	"github.com/msvens/mexif"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/model"
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
	dbs model.DataStore
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
	if ps.dbs, err = model.NewDB(); err != nil {
		logger.Errorw("could not create dbservice", "error", err)
		return nil, err
	}
	//Tables creating is moved to a separate command
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

func (ps *PhotoService) GetExif(id string, loogedIn bool) (*model.Exif, bool) {
	if exif, err := ps.dbs.Exif(id); err == nil {
		return exif, true
	} else {
		logger.Errorw("could get exif", "error", err)
		return nil, false
	}
}

func (ps *PhotoService) GetPhoto(id string, private bool) (*model.Photo, bool) {
	if p, err := ps.dbs.Photo(id, private); err == nil {
		return p, true
	} else {
		logger.Errorw("could not get photo", "error", err)
		return nil, false
	}
}

func (ps *PhotoService) GetPhotoAlbums(id string, private bool) ([]string, error) {
	if albums, err := ps.dbs.Albums(); err != nil {
		return nil, err
	} else {
		var names []string
		for _, a := range albums {
			names = append(names, a.Name)
		}
		return names, nil
	}
}

func (ps *PhotoService) GetLatestPhoto(private bool) (*model.Photo, bool) {
	r := model.Range{Offset: 0, Limit: 1}
	if photos, err := ps.dbs.Photos(r, model.DriveDate, model.PhotoFilter{Private: private}); err != nil {
		return nil, false
	} else if len(photos) > 1 {
		return nil, false
	} else {
		return photos[0], true
	}
}

func (ps *PhotoService) GetPhotos(sortOrder model.PhotoOrder, limit int, offset int, private bool) (*PhotoFiles, error) {
	r := model.Range{Offset: offset, Limit: limit}
	f := model.PhotoFilter{Private: private}
	if photos, err := ps.dbs.Photos(r, sortOrder, f); err != nil {
		return nil, err
	} else {
		return &PhotoFiles{Length: len(photos), Photos: photos}, nil
	}
}

func (ps *PhotoService) SearchByCameraModel(cameraModel string, private bool) (*PhotoFiles, error) {
	f := model.PhotoFilter{private, cameraModel}
	if photos, err := ps.dbs.Photos(model.Range{}, model.DriveDate, f); err != nil {
		return nil, err
	} else {
		return &PhotoFiles{Length: len(photos), Photos: photos}, nil
	}
}

func (ps *PhotoService) GetAlbumCollection(name string, private bool) (*AlbumCollection, error) {
	if album, err := ps.dbs.Album(name); err != nil {
		return nil, err
	} else {
		photos, err := ps.dbs.AlbumPhotos(name, model.PhotoFilter{Private: private})
		if err != nil {
			return nil, err
		}
		return &AlbumCollection{Info: album, Photos: &PhotoFiles{len(photos), photos}}, nil
	}
}

func (ps *PhotoService) GetAlbums() ([]*model.Album, error) {
	return ps.dbs.Albums()
}

func (ps *PhotoService) GetUser() (*model.User, error) {
	return ps.dbs.User()
}

func (ps *PhotoService) UpdateAlbum(description string, coverPic string, name string) (*model.Album, error) {
	a := model.Album{Name: name, Description: description, CoverPic: coverPic}
	return ps.dbs.UpdateAlbum(&a)
}

func (ps *PhotoService) UpdatePhoto(driveId string, title string, description string,
	keywords []string, albums []string) (*model.Photo, error) {
	if photo, err := ps.dbs.UpdatePhoto(title, description, keywords, driveId); err != nil {
		return nil, err
	} else {
		if err := ps.dbs.UpdatePhotoAlbums(albums, driveId); err != nil {
			return nil, err
		}
		return photo, err
	}
}

func (ps *PhotoService) TogglePrivate(photo *model.Photo) (*model.Photo, error) {
	return ps.dbs.SetPrivatePhoto(!photo.Private, photo.DriveId)
}

func (ps *PhotoService) UpdateUserDrive(name string) (*model.User, error) {
	if f, err := ps.DriveSrv.GetByName(name, true, false, fileFields); err != nil {
		return nil, err
	} else {
		if user, err := ps.dbs.User(); err != nil {
			return nil, InternalError(err.Error())
		} else {
			user.DriveFolderId = f.Id
			user.DriveFolderName = f.Name
			return ps.dbs.UpdateUser(user)
		}
	}
}

func (ps *PhotoService) UpdateUserPic(picUrl string) (*model.User, error) {
	if user, err := ps.dbs.User(); err != nil {
		return nil, InternalError(err.Error())
	} else {
		user.Pic = picUrl
		return ps.dbs.UpdateUser(user)
	}
}

func (ps *PhotoService) UpdateUser(user *model.User) (*model.User, error) {
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
		if !ps.dbs.HasPhoto(f.Id, true) {
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

func toFileName(driveId string) string {
	return driveId + ".jpg"
}

func (ps *PhotoService) AddPhoto(f *drive.File, tool *mexif.MExifTool) (bool, error) {
	var err error
	if ps.dbs.HasPhoto(f.Id, true) {
		return false, nil
	}
	photo := model.Photo{}
	photo.DriveId = f.Id
	//photo.Title = f.Name
	photo.Md5 = f.Md5Checksum
	photo.FileName = toFileName(f.Id)
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
	photo.Private = true
	photo.Likes = 0

	if err = ps.dbs.AddPhoto(&photo, exif); err != nil {
		logger.Errorw("error adding photo: ", zap.Error(err))
		return false, err
	}
	logger.Infow("added photo", "driveId", photo.DriveId)
	return true, nil
}

func (ps *PhotoService) DeletePhotos(removeFiles bool) (*PhotoFiles, error) {
	logger.Infow("Delete All Photos", "removeFiles", removeFiles)
	if photos, err := ps.dbs.Photos(model.Range{}, model.None, model.PhotoFilter{Private: true}); err != nil {
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

func (ps *PhotoService) DeletePhoto(p *model.Photo, removeFiles bool) (*model.Photo, error) {
	logger.Infow("Delete Photo", "id", p.DriveId, "removeFiles", removeFiles)
	if del, err := ps.dbs.DeletePhoto(p.DriveId); err != nil {
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

func (ps *PhotoService) downloadPhoto(photo *model.Photo) error {

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
