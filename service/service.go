package service

import (
	"encoding/json"
	"fmt"
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

	fileFields = "id, name, kind, mimeType, md5Checksum, createdTime"
)

var (
	//srvPath string
)

type PhotoService struct {
	DriveSrv *mdrive.DriveService
	driveFolder *drive.File
	rootDir string
	imgDir string
	thumbDir string
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
	//ps.imgDir = srvPath + "/img/"
	//ps.thumbDir = srvPath + "/thumb/"
	if err := ps.createPaths(); err != nil {
		log.Println("could not create image folders")
	}
	ps.DriveSrv = driveSrv
	ps.folderPath = ps.rootDir+"/"+folderFileName
	//ps.photoList = &PhotoList{make(map[string]*Photo)}
	//ps.readPhotos()
	f, err := readDriveFolder(ps.folderPath)
	if err != nil {
		log.Println("could not read photo folder: ", ps.folderPath, err)
	} else {
		ps.driveFolder = f
	}
	//Open db
	if ps.dbs, err = NewDbService(); err != nil {
		log.Println(err)
	} else {
		err = ps.dbs.CreateTables()
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

func (ps *PhotoService) GetPhotoFolder() *drive.File {
	return ps.driveFolder
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
	//ps.dbs.GetId(id)
	if p, err := ps.dbs.GetId(id); err == nil {
		return p, true
	} else {
		log.Println(err)
		return nil, false
	}
	/*val, found :=  ps.photoList.Photos[id]
	return val, found*/
}

/*
func (ps *PhotoService) GetPhotoList() *PhotoList {
	return ps.photoList
}
*/

func (ps *PhotoService) GetPhotos() ([]*Photo, error) {

	return ps.dbs.List()
	/*
	photos := make([]*Photo, 0, len(ps.photoList.Photos))
	for _, v := range ps.photoList.Photos {
		photos = append(photos, v)
	}
	return photos
	 */
}

func (ps *PhotoService) GetImgPath(fileName string) string {
	return filepath.Join(ps.imgDir, fileName)
}

func (ps *PhotoService) GetThumbPath(fileName string) string {
	return filepath.Join(ps.thumbDir, fileName)
}

func (ps *PhotoService) SetDriveFolderByName(name string) (*drive.File, error) {

	f, err := ps.DriveSrv.GetByName(name, true, false, fileFields)

	if err != nil {
		return nil, err
	}
	err = ps.SetDriveFolder(f)
	return f, err
}

func (ps *PhotoService) SetDriveFolderId(id string) (*drive.File, error) {
	f, err := ps.DriveSrv.Get(id)
	if err != nil {
		return nil, err
	}
	err = ps.SetDriveFolder(f)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (ps *PhotoService) SetDriveFolder(file *drive.File) error {
	err := saveDriveFolder(file, ps.folderPath)
	if err != nil {
		return err
	}
	ps.driveFolder = file
	return nil
}

func (ps *PhotoService) ListDriveByName(name string) ([]*drive.File, error) {
	f, err := ps.DriveSrv.GetByName(name, true, false, fileFields)
	if err != nil {
		return nil, err
	}
	query := mdrive.NewQuery().Parents().In(f.Id).And().MimeType().Eq(mdrive.Jpeg).TrashedEq(false)
	return ps.DriveSrv.SearchAll(query, fileFields)
}

func (ps *PhotoService) ListDriveById(id string) ([]*drive.File, error) {
	query := mdrive.NewQuery().Parents().In(id).And().MimeType().Eq(mdrive.Jpeg).TrashedEq(false)
	return ps.DriveSrv.SearchAll(query, fileFields)
}

func (ps *PhotoService) ListDriveFolder() ([]*drive.File, error) {
	return ps.ListDriveById(ps.driveFolder.Id)
}

func (ps *PhotoService) ListNNewPhotos() ([]*drive.File, error) {
	fl, err := ps.ListDriveFolder()
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

func (ps *PhotoService) AddPhotos() error {
	fl, err := ps.ListDriveFolder()
	if err != nil {
		return err
	}

	tool, err := mexif.NewMExifTool()
	if err != nil {
		return err
	}

	for _, f := range fl {
		err = ps.AddPhoto(f, tool)
		if err != nil {
			return err
		}
	}
	err = tool.Close()
	if err != nil {
		log.Println(err)
	}
	return nil
	//return ps.savePhotos()
}

func (ps *PhotoService) AddPhoto(f *drive.File, tool *mexif.MExifTool) error {
	var err error
	if ps.dbs.Contains(f.Id) {
		log.Println("photo already existed")
		return nil
	}
	photo := Photo{}
	photo.DriveId = f.Id
	photo.Title = f.Name
	photo.Md5 = f.Md5Checksum
	photo.FileName = f.Id+".jpg"
	if t, err := mdrive.ParseTime(f.CreatedTime); err == nil {
		photo.DriveDate = t
	}

	if err = ps.downloadPhoto(&photo); err != nil {
		log.Println("error downloading: ", err)
		return err
	}
	var exif *mexif.ExifCompact

	if exif, err = tool.ExifCompact(ps.GetImgPath(photo.FileName)); err == nil {
		photo.CameraMake = exif.CameraMake
		photo.CameraModel = exif.CameraModel
		photo.LensMake = exif.LensMake
		photo.LensModel = exif.LensModel
		photo.Exposure = exif.ExposureTime
		photo.FNumber = exif.FNumber
		photo.Iso = exif.ISO
		if exif.Title != "" {
			photo.Title = exif.Title
		}
		if len(exif.Keywords) > 0 {
			photo.Keywords = strings.Join(exif.Keywords, ",")
		}
		photo.OriginalDate = exif.OriginalDate
	} else {
		return err
	}

	if err = ps.dbs.AddPhoto(&photo, exif); err != nil {
		log.Println("error adding photo: ", err)
		return err
	}
	log.Println("added photo: ", photo.Title)
	return nil
}

func (ps *PhotoService) downloadPhoto(photo *Photo) error{

	if _, err := ps.DriveSrv.Download(photo.DriveId, ps.GetImgPath(photo.FileName)); err != nil {
		return err;
	}
	//create thumbnail

	args := []string{ps.GetImgPath(photo.FileName), "-s", "640", "-m", "centre", "-o", ps.GetThumbPath(photo.FileName)}
	log.Println("creating thumbnail", strings.Join(args, " "))
	cmd := exec.Command("vipsthumbnail", args...)

	if err := cmd.Start(); err != nil {
		fmt.Println("error creating thumbnail ", err)
		return err
	}
	return nil
}

func readDriveFolder(folderFile string) (*drive.File, error) {
	f, err := os.OpenFile(folderFile, os.O_RDONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	folder := drive.File{}
	err = json.NewDecoder(f).Decode(&folder)
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

func saveDriveFolder(file *drive.File, folderFile string) error{
	log.Printf("Saving photos folder to: %s\n", folderFile)
	f, err := os.OpenFile(folderFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to folder file: %v", err)
	}
	defer f.Close()
	err = json.NewEncoder(f).Encode(file)
	if err != nil {
		return fmt.Errorf("unable to encode folder: %v", err)
	}
	return nil
}


