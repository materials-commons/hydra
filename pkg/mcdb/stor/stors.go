package stor

import (
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type ConversionStor interface {
	AddFileToConvert(file *mcmodel.File) (*mcmodel.Conversion, error)
}

type FileStor interface {
	GetFileByID(fileID int) (*mcmodel.File, error)
	GetFileByUUID(fileUUID string) (*mcmodel.File, error)
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
	DoneWritingToFile(file *mcmodel.File, checksum string, size int64, conversionStore ConversionStor) (bool, error)
}

type ProjectStor interface {
	CreateProject(project *mcmodel.Project) (*mcmodel.Project, error)
	GetProjectByID(projectID int) (*mcmodel.Project, error)
	GetProjectBySlug(slug string) (*mcmodel.Project, error)
	GetProjectsForUser(userID int) ([]mcmodel.Project, error)
	UpdateProjectSizeAndFileCount(projectID int, size int64, fileCount int) error
	UpdateProjectDirectoryCount(projectID int, directoryCount int) error
	UserCanAccessProject(userID, projectID int) bool
}

type TransferRequestFileStor interface {
	DeleteTransferFileRequestByPath(ownerID, projectID int, path string) error
	GetTransferFileRequestByPath(ownerID, projectID int, path string) (*mcmodel.TransferRequestFile, error)
	DeleteTransferRequestFile(transferRequestFile *mcmodel.TransferRequestFile) error
}

type TransferRequestStor interface {
	CreateTransferRequest(tr *mcmodel.TransferRequest) (*mcmodel.TransferRequest, error)
	ListTransferRequests() ([]mcmodel.TransferRequest, error)
	MarkFileReleased(file *mcmodel.File, checksum string, projectID int, totalBytes int64) error
	MarkFileAsOpen(file *mcmodel.File) error
	CreateNewFile(file, dir *mcmodel.File, transferRequest *mcmodel.TransferRequest) (*mcmodel.File, error)
	CreateNewFileVersion(file, dir *mcmodel.File, transferRequest *mcmodel.TransferRequest) (*mcmodel.File, error)
	ListDirectory(dir *mcmodel.File, transferRequest *mcmodel.TransferRequest) ([]mcmodel.File, error)
	GetFileByPath(path string, transferRequest *mcmodel.TransferRequest) (*mcmodel.File, error)
	GetTransferRequestByProjectAndUser(projectID, userID int) (*mcmodel.TransferRequest, error)
}

type GlobusTransferStor interface {
	CreateGlobusTransfer(globusTransfer *mcmodel.GlobusTransfer) (*mcmodel.GlobusTransfer, error)
}

type UserStor interface {
	CreateUser(user *mcmodel.User) (*mcmodel.User, error)
	GetUsersWithGlobusAccount() ([]mcmodel.User, error)
	GetUserBySlug(slug string) (*mcmodel.User, error)
}

type Stors struct {
	ConversionStor          ConversionStor
	FileStor                FileStor
	ProjectStor             ProjectStor
	TransferRequestFileStor TransferRequestFileStor
	TransferRequestStor     TransferRequestStor
	GlobusTransferStor      GlobusTransferStor
	UserStor                UserStor
}

func NewGormStors(db *gorm.DB, mcfsRoot string) *Stors {
	return &Stors{
		ConversionStor:          NewGormConversionStor(db),
		FileStor:                NewGormFileStor(db, mcfsRoot),
		ProjectStor:             NewGormProjectStor(db),
		TransferRequestFileStor: NewGormTransferRequestFileStor(db),
		TransferRequestStor:     NewGormTransferRequestStor(db, mcfsRoot),
		GlobusTransferStor:      NewGormGlobusTransferStor(db),
		UserStor:                NewGormUserStor(db),
	}
}
