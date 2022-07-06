package store

import "github.com/materials-commons/gomcdb/mcmodel"

type ConversionStore interface {
	AddFileToConvert(file *mcmodel.File) (*mcmodel.Conversion, error)
}

type FileStore interface {
	UpdateMetadataForFileAndProject(file *mcmodel.File, checksum string, totalBytes int64) error
	CreateFile(name string, projectID, directoryID, ownerID int, mimeType string) (*mcmodel.File, error)
	GetDirByPath(projectID int, path string) (*mcmodel.File, error)
	CreateDirectory(parentDirID, projectID, ownerID int, path, name string) (*mcmodel.File, error)
	CreateDirIfNotExists(parentDirID int, path, name string, projectID, ownerID int) (*mcmodel.File, error)
	ListDirectoryByPath(projectID int, path string) ([]mcmodel.File, error)
	GetOrCreateDirPath(projectID, ownerID int, path string) (*mcmodel.File, error)
	GetFileByPath(projectID int, path string) (*mcmodel.File, error)
	UpdateFileUses(file *mcmodel.File, uuid string, fileID int) error
	PointAtExistingIfExists(file *mcmodel.File) (bool, error)
	DoneWritingToFile(file *mcmodel.File, checksum string, size int64, conversionStore ConversionStore) (bool, error)
}

type ProjectStore interface {
	GetProjectByID(projectID int) (*mcmodel.Project, error)
	GetProjectBySlug(slug string) (*mcmodel.Project, error)
	GetProjectsForUser(userID int) ([]mcmodel.Project, error)
	UpdateProjectSizeAndFileCount(projectID int, size int64, fileCount int) error
	UpdateProjectDirectoryCount(projectID int, directoryCount int) error
	UserCanAccessProject(userID, projectID int) bool
}

type TransferRequestFileStore interface {
	DeleteTransferFileRequestByPath(ownerID, projectID int, path string) error
	GetTransferFileRequestByPath(ownerID, projectID int, path string) (*mcmodel.TransferRequestFile, error)
	DeleteTransferRequestFile(transferRequestFile *mcmodel.TransferRequestFile) error
}

type TransferRequestStore interface {
	MarkFileReleased(file *mcmodel.File, checksum string, projectID int, totalBytes int64) error
	MarkFileAsOpen(file *mcmodel.File) error
	CreateNewFile(file, dir *mcmodel.File, transferRequest mcmodel.TransferRequest) (*mcmodel.File, error)
	CreateNewFileVersion(file, dir *mcmodel.File, transferRequest mcmodel.TransferRequest) (*mcmodel.File, error)
	ListDirectory(dir *mcmodel.File, transferRequest mcmodel.TransferRequest) ([]mcmodel.File, error)
	GetFileByPath(path string, transferRequest mcmodel.TransferRequest) (*mcmodel.File, error)
}

type UserStore interface {
	GetUsersWithGlobusAccount() ([]mcmodel.User, error)
	GetUserBySlug(slug string) (*mcmodel.User, error)
}
