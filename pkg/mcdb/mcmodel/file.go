package mcmodel

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type File struct {
	ID                      int       `json:"id"`
	UUID                    string    `json:"uuid"`
	UsesUUID                string    `json:"uses_uuid"`
	UsesID                  int       `json:"uses_id"`
	ProjectID               int       `json:"project_id"`
	Name                    string    `json:"name"`
	Description             string    `json:"description"`
	Summary                 string    `json:"summary"`
	OwnerID                 int       `json:"owner_id"`
	Path                    string    `json:"path"`
	DirectoryID             int       `json:"directory_id" gorm:"default:null"`
	DatasetID               int       `json:"dataset_id" gorm:"default:null"`
	Size                    uint64    `json:"size"`
	Checksum                string    `json:"checksum"`
	MimeType                string    `json:"mime_type"`
	MediaTypeDescription    string    `json:"media_type_description"`
	Current                 bool      `json:"current"`
	Directory               *File     `json:"directory" gorm:"foreignKey:DirectoryID;references:ID"`
	DeletedAt               time.Time `gorm:"default:null"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
	UploadSource            string    `json:"upload_source"`
	FileMissingAt           time.Time `json:"file_missing_at" gorm:"default:null"`
	FileMissingDeterminedBy string    `json:"file_missing_determined_by"`
	Health                  string    `json:"health"`
	HealthFixedBy           string    `json:"health_fixed_by"`
	ThumbnailCreatedAt      time.Time `json:"thumbnail_created_at" gorm:"default:null"`
	ThumbnailStatus         string    `json:"thumbnail_status"`
	ConversionCreatedAt     time.Time `json:"conversion_created_at" gorm:"default:null"`
	ConversionStatus        string    `json:"conversion_status"`
}

// ToFTSDocument converts a File to a map[string]any that can be used in an FTS document. This matches the fields
// used on the Laravel side. See app/Models/File.php->toSearchableArray() method for fields we are currently indexing.
// This needs to be kept in sync with the fields in the Laravel model.
func (f File) ToFTSDocument() map[string]any {
	fileType := "file"
	if f.MimeType == "directory" {
		fileType = "directory"
	}

	path := f.Path
	if path == "" {
		// f.Path is only set for directories. So if the Path is empty, then we know it's a file. Before setting
		// the path we need to make sure that f.Directory has been loaded. If not, then don't set the path.
		if f.Directory != nil {
			path = filepath.Join(f.Directory.Path, f.Name)
		}
	}

	// Make sure the fields here match the fields in the Laravel model: app/Models/File.php->toSearchableArray() method
	return map[string]any{
		"id":                     f.ID,
		"name":                   f.Name,
		"description":            f.Description,
		"current":                f.Current,
		"deleted_at":             f.DeletedAt,
		"dataset_id":             f.DatasetID,
		"directory_id":           f.DirectoryID,
		"path":                   path,
		"mime_type":              f.MimeType,
		"media_type_description": f.MediaTypeDescription,
		"project_id":             f.ProjectID,
		"summary":                f.Summary,
		"type":                   fileType,
	}
}

// MkdirUnderlyingPath creates the directory to store the actual file. It takes into account the actual file path,
// which is derived from the uuid, or uses_uuid, if that is set.
func (f File) MkdirUnderlyingPath(mcfsDir string) error {
	return os.MkdirAll(f.ToUnderlyingDirPath(mcfsDir), 0755)
}

// MkdirUnderlyingPathForUUID creates a directory path given an uuid. This can be used in place of MkdirUnderlyingPath
// when you need to ensure the file goes to a specific UUID, rather than the one derived from the files uuid or uses_uuid.
func (f File) MkdirUnderlyingPathForUUID(mcfsDir string) error {
	return os.MkdirAll(f.ToUnderlyingDirPathForUUID(mcfsDir), 0755)
}

// CreateUnderlyingFile creates a file then immediately closes it. Use it to create an empty file and a file objects
// underlying path.
func (f File) CreateUnderlyingFile(mcfsDir string) error {
	handle, err := f.CreateReturningHandleToUnderlyingFile(mcfsDir)
	if err != nil {
		return err
	}

	_ = handle.Close()

	return nil
}

// CreateReturningHandleToUnderlyingFile creates the file at its underlying path and returns the handle to it.
func (f File) CreateReturningHandleToUnderlyingFile(mcfsDir string) (*os.File, error) {
	if err := f.MkdirUnderlyingPath(mcfsDir); err != nil {
		return nil, err
	}

	handle, err := os.Create(f.ToUnderlyingFilePath(mcfsDir))
	if err != nil {
		return nil, err
	}

	return handle, nil
}

// TableName returns the name of the table Gorm uses for the file model.
func (File) TableName() string {
	return "files"
}

// IsFile returns true if the file is a file and not a directory. It does this by checking if MimeType is not "directory".
func (f File) IsFile() bool {
	return f.MimeType != "directory"
}

// IsDir returns true if the file is a directory and not a file. It does this by checking if MimeType is "directory".
func (f File) IsDir() bool {
	return f.MimeType == "directory"
}

// FullPath returns the full path of the file. If the file is a directory, it returns the directory path.
// If the file is a file, it returns the file path. NOTE: This assumes that f.Directory is not nil.
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

// RealFileExists returns true if the file exists at its underlying path. It does this by checking if the file
// exists at its underlying path.
func (f File) RealFileExists(mcdir string) bool {
	_, err := os.Stat(f.ToUnderlyingFilePath(mcdir))
	if err != nil {
		return false
	}

	return true
}

// ToUnderlyingFilePath returns the underlying storage path to a file.
func (f File) ToUnderlyingFilePath(mcdir string) string {
	return filepath.Join(f.ToUnderlyingDirPath(mcdir), f.UUIDForPath())
}

// ToUnderlyingFilePathForUUID returns the path for a file given its UUID.
func (f File) ToUnderlyingFilePathForUUID(mcdir string) string {
	uuidParts := strings.Split(f.UUID, "-")
	return filepath.Join(mcdir, uuidParts[1][0:2], uuidParts[1][2:4], f.UUID)
}

// ToUnderlyingDirPath returns the underlying directory path for a file.
func (f File) ToUnderlyingDirPath(mcdir string) string {
	uuidParts := strings.Split(f.UUIDForPath(), "-")
	return filepath.Join(mcdir, uuidParts[1][0:2], uuidParts[1][2:4])
}

// ToUnderlyingDirPathForUUID returns the underlying directory path for a file given its UUID.
func (f File) ToUnderlyingDirPathForUUID(mcdir string) string {
	uuidParts := strings.Split(f.UUID, "-")
	return filepath.Join(mcdir, uuidParts[1][0:2], uuidParts[1][2:4])
}

// UUIDForPath returns the UUID for a file given its path. It checks if the UsesUUID is set, and uses it if it is.
// Otherwise, it returns the UUID.
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

// IsConvertible checks if a file is convertible. Convertible means that we can run a converter on it to display
// the file on the web.
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
	case "application/json":
		// Handle Jupyter Notebooks
		if strings.HasSuffix(f.Name, ".ipynb") {
			return true
		}
		return false
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
