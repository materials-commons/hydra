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
	UpdateFile(file, updates *mcmodel.File) (*mcmodel.File, error)
	SetUsesToNull(file *mcmodel.File) (*mcmodel.File, error)
	SetFileAsCurrent(file *mcmodel.File) (*mcmodel.File, error)
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
	GetMatchingFileInDirectory(directoryID int, checksum string, name string) (*mcmodel.File, error)
	SetFileHealthMissing(file *mcmodel.File, determinedBy string, source string) (*mcmodel.File, error)
	SetFileHealthFixed(file *mcmodel.File, fixedBy string, source string) (*mcmodel.File, error)
	FindMatchingFileByChecksum(checksum string) (*mcmodel.File, error)
	Root() string
}

type ProjectStor interface {
	CreateProject(project *mcmodel.Project) (*mcmodel.Project, error)
	GetProjectByID(projectID int) (*mcmodel.Project, error)
	GetProjectBySlug(slug string) (*mcmodel.Project, error)
	GetProjectsForUser(userID int) ([]mcmodel.Project, error)
	UpdateProjectSizeAndFileCount(projectID int, size int64, fileCount int) error
	UpdateProjectDirectoryCount(projectID int, directoryCount int) error
	UserCanAccessProject(userID, projectID int) bool
	AddMemberToProject(project *mcmodel.Project, user *mcmodel.User) error
	AddAdminToProject(project *mcmodel.Project, user *mcmodel.User) error
}

type TransferRequestFileStor interface {
	DeleteTransferFileRequestByPath(ownerID, projectID int, path string) error
	GetTransferFileRequestByPath(ownerID, projectID int, path string) (*mcmodel.TransferRequestFile, error)
	GetTransferRequestFileByPathForTransferRequest(path string, transferRequest *mcmodel.TransferRequest) (*mcmodel.TransferRequestFile, error)
	DeleteTransferRequestFile(transferRequestFile *mcmodel.TransferRequestFile) error
}

type TransferRequestStor interface {
	CreateTransferRequest(tr *mcmodel.TransferRequest) (*mcmodel.TransferRequest, error)
	ListTransferRequests() ([]mcmodel.TransferRequest, error)
	MarkFileReleased(file *mcmodel.File, checksum string, projectID int, totalBytes int64) error
	MarkFileAsOpen(file *mcmodel.File) error
	CreateNewFile(file, dir *mcmodel.File, transferRequest *mcmodel.TransferRequest) (*mcmodel.File, *mcmodel.TransferRequestFile, error)
	CreateNewFileVersion(file, dir *mcmodel.File, transferRequest *mcmodel.TransferRequest) (*mcmodel.File, error)
	ListDirectory(dir *mcmodel.File, transferRequest *mcmodel.TransferRequest) ([]mcmodel.File, error)
	GetFileByPath(path string, transferRequest *mcmodel.TransferRequest) (*mcmodel.File, error)
	GetTransferRequestForProjectAndUser(projectID, userID int) (*mcmodel.TransferRequest, error)
	GetTransferRequestsForProject(projectID int) ([]mcmodel.TransferRequest, error)
	GetTransferRequestByUUID(transferUUID string) (*mcmodel.TransferRequest, error)
	CloseTransferRequestByUUID(transferUUID string) error
}

type GlobusTransferStor interface {
	CreateGlobusTransfer(globusTransfer *mcmodel.GlobusTransfer) (*mcmodel.GlobusTransfer, error)
	GetGlobusTransferByGlobusIdentityID(globusIdentityID string) (*mcmodel.GlobusTransfer, error)
}

type RemoteClientStor interface {
	CreateRemoteClient(RemoteClient *mcmodel.RemoteClient) (*mcmodel.RemoteClient, error)
	GetRemoteClientByClientID(clientID string) (*mcmodel.RemoteClient, error)
}

type RemoteClientTransferStor interface {
	CreateRemoteClientTransfer(clientTransfer *mcmodel.RemoteClientTransfer) (*mcmodel.RemoteClientTransfer, error)
	GetRemoteClientTransferByUUID(clientUUID string) (*mcmodel.RemoteClientTransfer, error)
	UpdateRemoteClientTransferState(UUID string, state string) (*mcmodel.RemoteClientTransfer, error)
	GetAllTransfersForRemoteClient(remoteClientID int) ([]mcmodel.RemoteClientTransfer, error)
	GetAllUploadTransfersForRemoteClient(remoteClientID int) ([]mcmodel.RemoteClientTransfer, error)
	GetAllDownloadTransfersForRemoteClient(remoteClientID int) ([]mcmodel.RemoteClientTransfer, error)
}

type UserStor interface {
	CreateUser(user *mcmodel.User) (*mcmodel.User, error)
	GetUsersWithGlobusAccount() ([]mcmodel.User, error)
	GetUserBySlug(slug string) (*mcmodel.User, error)
	GetUserByEmail(email string) (*mcmodel.User, error)
	GetUserByAPIToken(apitoken string) (*mcmodel.User, error)
}

//type ClientTransferStor interface {
//	CreateClientTransfer(ct *mcmodel.ClientTransfer) (*mcmodel.ClientTransfer, error)
//	GetOrCreateClientTransferByPath(clientUUID string, projectID, ownerID int, filePath string) (*mcmodel.ClientTransfer, *mcmodel.TransferRequestFile, error)
//	UpdateClientTransfer(ct *mcmodel.ClientTransfer) (*mcmodel.ClientTransfer, error)
//	CloseClientTransfer(clientTransferID int) error
//	AbortClientTransfer(clientTransferID int) error
//}

type Stors struct {
	ConversionStor           ConversionStor
	FileStor                 FileStor
	ProjectStor              ProjectStor
	TransferRequestFileStor  TransferRequestFileStor
	TransferRequestStor      TransferRequestStor
	GlobusTransferStor       GlobusTransferStor
	UserStor                 UserStor
	RemoteClientStor         RemoteClientStor
	RemoteClientTransferStor RemoteClientTransferStor
}

func NewGormStors(db *gorm.DB, mcfsRoot string) *Stors {
	return &Stors{
		ConversionStor:           NewGormConversionStor(db),
		FileStor:                 NewGormFileStor(db, mcfsRoot),
		ProjectStor:              NewGormProjectStor(db),
		TransferRequestFileStor:  NewGormTransferRequestFileStor(db),
		TransferRequestStor:      NewGormTransferRequestStor(db, mcfsRoot),
		GlobusTransferStor:       NewGormGlobusTransferStor(db),
		UserStor:                 NewGormUserStor(db),
		RemoteClientStor:         NewGormRemoteClientStor(db),
		RemoteClientTransferStor: NewGormRemoteClientTransferStor(db),
	}
}
