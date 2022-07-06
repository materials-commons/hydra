package mcmodel

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type File struct {
	ID                   int       `json:"id"`
	UUID                 string    `json:"uuid"`
	UsesUUID             string    `json:"uses_uuid"`
	UsesID               int       `json:"uses_id"`
	ProjectID            int       `json:"project_id"`
	Name                 string    `json:"name"`
	OwnerID              int       `json:"owner_id"`
	Path                 string    `json:"path"`
	DirectoryID          int       `json:"directory_id"`
	Size                 uint64    `json:"size"`
	Checksum             string    `json:"checksum"`
	MimeType             string    `json:"mime_type"`
	MediaTypeDescription string    `json:"media_type_description"`
	Current              bool      `json:"current"`
	Directory            *File     `json:"directory" gorm:"foreignKey:DirectoryID;references:ID"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (File) TableName() string {
	return "files"
}

func (f File) IsFile() bool {
	return f.MimeType != "directory"
}

func (f File) IsDir() bool {
	return f.MimeType == "directory"
}

func (f File) FullPath() string {
	if f.IsDir() {
		return f.Path
	}

	// f is a file and not a directory
	if f.Directory.Path == "/" {
		return f.Directory.Path + f.Name
	}

	return f.Directory.Path + "/" + f.Name
}

func (f File) ToUnderlyingFilePath(mcdir string) string {
	return filepath.Join(f.ToUnderlyingDirPath(mcdir), f.UUIDForPath())
}

func (f File) ToUnderlyingFilePathForUUID(mcdir string) string {
	uuidParts := strings.Split(f.UUID, "-")
	return filepath.Join(mcdir, uuidParts[1][0:2], uuidParts[1][2:4], f.UUID)
}

func (f File) ToUnderlyingDirPath(mcdir string) string {
	uuidParts := strings.Split(f.UUIDForPath(), "-")
	return filepath.Join(mcdir, uuidParts[1][0:2], uuidParts[1][2:4])
}

func (f File) UUIDForPath() string {
	if f.UsesUUID != "" {
		return f.UsesUUID
	}

	return f.UUID
}

func (f File) IDForUses() int {
	if f.UsesID != 0 {
		return f.UsesID
	}

	return f.ID
}

func (f File) UUIDForUses() string {
	return f.UUIDForPath()
}

func (f File) IsConvertible() bool {
	switch f.MimeType {
	case "application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation":
		// Office documents that can be converted to PDF
		return true
	case "image/bmp",
		"image/x-ms-bmp",
		"image/tiff":
		// images that need to be converted to JPEG to display on web
		return true
	default:
		return false
	}
}

//////////////////////////////////////////

type FileInfo struct {
	file File
}

func (f File) ToFileInfo() FileInfo {
	return FileInfo{file: f}
}

func (f FileInfo) Name() string {
	return f.file.Name
}

func (f FileInfo) Size() int64 {
	return int64(f.file.Size)
}

func (f FileInfo) Mode() fs.FileMode {
	if f.file.IsDir() {
		return os.FileMode(0777) | os.ModeDir
	}

	return fs.FileMode(0777)
}

func (f FileInfo) ModTime() time.Time {
	return f.file.UpdatedAt
}

func (f FileInfo) IsDir() bool {
	return f.file.IsDir()
}

func (f FileInfo) Sys() interface{} {
	return nil
}

////////////////////////////////////

type DirEntry struct {
	finfo FileInfo
}

func (f File) ToDirEntry() DirEntry {
	return DirEntry{finfo: f.ToFileInfo()}
}

func (d DirEntry) Name() string {
	return d.finfo.Name()
}

func (d DirEntry) IsDir() bool {
	return d.finfo.IsDir()
}

func (d DirEntry) Type() fs.FileMode {
	return d.finfo.Mode()
}

func (d DirEntry) Info() (fs.FileInfo, error) {
	return d.finfo, nil
}
