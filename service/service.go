package service

import (
	"github.com/msvens/mdrive"
	"github.com/msvens/mexif"
	"github.com/msvens/mphotos/config"
	"google.golang.org/api/drive/v3"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

func NewPhotosService(driveSrv *mdrive.DriveService) *PhotoService {
	//srvPath = config.ServiceRoot()
	ps := PhotoService{}
	ps.rootDir = config.ServiceRoot()
	ps.imgDir = config.ServicePath("img")
	ps.thumbDir = config.ServicePath("thumb")
	if err := ps.createPaths(); err != nil {
		log.Println("could not create image folders")
	}
	ps.DriveSrv = driveSrv
	ps.folderPath = ps.rootDir + "/" + folderFileName
	/*f, err := readDriveFolder(ps.folderPath)
	if err != nil {
		log.Println("could not read photo folder: ", ps.folderPath, err)
	} else {
		ps.driveFolder = f
	}*/
	//Open db
	var err error
	if ps.dbs, err = NewDbService(); err != nil {
		log.Println(err)
	} else {
		if err = ps.dbs.CreateTables(); err != nil {
			log.Println(err)
		}

	}
	return &ps
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
		log.Println(err)
		return nil, false
	}
}

func (ps *PhotoService) GetPhoto(id string) (*Photo, bool) {
	if p, err := ps.dbs.GetId(id); err == nil {
		return p, true
	} else {
		log.Println(err)
		return nil, false
	}
}

func (ps *PhotoService) GetLatestPhoto() (*Photo, bool) {
	if p, err := ps.dbs.GetLatest(); err == nil {
		return p, true
	} else {
		log.Println(err)
		return nil, false
	}
}

func (ps *PhotoService) GetPhotos(driveDate bool, limit int, offset int) (*PhotoFiles, error) {
	var err error
	var photos []*Photo
	if driveDate {
		photos, err = ps.dbs.GetByDriveDate(limit, offset)
	} else {
		photos, err = ps.dbs.GetByOriginalDate(limit, offset)
	}
	if err != nil {
		return nil, err
	}
	return &PhotoFiles{Length: len(photos), Photos: photos}, nil
}

func (ps *PhotoService) GetUser() (*User, error) {
	return ps.dbs.GetUser()
}

func (ps *PhotoService) UpdateUser(user *User, cols []string) (*User, error) {
	if len(cols) == 0 { //all fields should be updated
		return ps.dbs.UpdateUser(user)
	} else {
		var err error
		for _, col := range cols {
			switch col {
			case "bio":
				_, err = ps.dbs.UpdateUserBio(user.Bio)
			case "name":
				_, err = ps.dbs.UpdateUserName(user.Name)
			case "pic":
				_, err = ps.dbs.UpdateUserPic(user.Pic)
			default:
				return nil, mdrive.NewError(mdrive.ErrorBadRequest, "no such field")
			}
			if err != nil {
				return nil, err
			}
		}
		return ps.GetUser()
	}
}

func (ps *PhotoService) GetImgPath(fileName string) string {
	return filepath.Join(ps.imgDir, fileName)
}

func (ps *PhotoService) GetThumbPath(fileName string) string {
	return filepath.Join(ps.thumbDir, fileName)
}

func (ps *PhotoService) UpdateDriveFolder(id string, name string) (*User, error) {
	if id == "" {
		if f, err := ps.DriveSrv.GetByName(name, true, false, fileFields); err != nil {
			return nil, err
		} else {
			return ps.dbs.UpdateUserDriveFolder(f.Id, f.Name)
		}
	} else {
		if f, err := ps.DriveSrv.Get(id); err != nil {
			return nil, err
		} else {
			return ps.dbs.UpdateUserDriveFolder(f.Id, f.Name)
		}
	}
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
		return nil, NewError(ApiErrorBackendError, "user not found")
	} else if u.DriveFolderId == "" {
		return nil, NewError(ApiErrorNotFound, "Drive folder has not been set")
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
	err = tool.Close()
	if err != nil {
		log.Println(err)
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
		log.Println("error downloading: ", err)
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
		log.Println("error adding photo: ", err)
		return false, err
	}
	log.Println("added photo: ", photo.Title)
	return true, nil
}

func (ps *PhotoService) DeletePhotos(removeFiles bool) (*PhotoFiles, error) {
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
	if del, err := ps.dbs.Delete(p.DriveId); err != nil {
		return nil, err
	} else if !del {
		return nil, NewError(ApiErrorNotFound, "Photo not found")
	}
	if !removeFiles {
		return p, nil
	}
	//remove files
	if err := os.Remove(ps.GetImgPath(p.FileName)); err != nil {
		return nil, ResolveError(err)
	}
	if err := os.Remove(ps.GetThumbPath(p.FileName)); err != nil {
		return nil, ResolveError(err)
	}
	return p, nil

}

func (ps *PhotoService) downloadPhoto(photo *Photo) error {

	if _, err := ps.DriveSrv.Download(photo.DriveId, ps.GetImgPath(photo.FileName)); err != nil {
		return err
	}

	//create thumbnail
	//args := []string{ps.GetImgPath(photo.FileName), "-s", "640", "-m", "centre", "-o", ps.GetThumbPath(photo.FileName)}
	args := []string{ps.GetImgPath(photo.FileName), "-s", "640", "-c", "-o", ps.GetThumbPath(photo.FileName)}
	log.Println("creating thumbnail", strings.Join(args, " "))
	cmd := exec.Command("vipsthumbnail", args...)

	if err := cmd.Start(); err != nil {
		return NewError(ApiErrorBackendError, err.Error())
	}

	return nil
}
