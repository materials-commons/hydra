package mcfs

import (
	"context"
	"log/slog"
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/materials-commons/hydra/pkg/mcbridgefs/fs/mcfs/projectpath"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type OpenerCreater interface {
	Open(ctx context.Context, path string, flags uint32) (fh fs.FileHandle, errno syscall.Errno)
	Create(ctx context.Context, path string, flags, mode uint32) (fh fs.FileHandle, errno syscall.Errno)
}

type OpenerCreaterFactory interface {
	NewOpenerCreaterHandler() OpenerCreater
}

type LocalOpenCreateHandlerFactory struct {
	knownFilesTracker   *KnownFilesTracker
	transferRequestStor stor.TransferRequestStor
	fileStor            stor.FileStor
	fileHandleFactory   FileHandleFactory
}

func NewLocalOpenCreateHandlerFactory() *LocalOpenCreateHandlerFactory {
	return &LocalOpenCreateHandlerFactory{}
}

func (h *LocalOpenCreateHandlerFactory) WithKnownFileTracker(knownFilesTracker *KnownFilesTracker) *LocalOpenCreateHandlerFactory {
	h.knownFilesTracker = knownFilesTracker
	return h
}

func (h *LocalOpenCreateHandlerFactory) WithTransferRequestStor(transferRequestStor stor.TransferRequestStor) *LocalOpenCreateHandlerFactory {
	h.transferRequestStor = transferRequestStor
	return h
}

func (h *LocalOpenCreateHandlerFactory) WithFileStor(fileStor stor.FileStor) *LocalOpenCreateHandlerFactory {
	h.fileStor = fileStor
	return h
}

func (h *LocalOpenCreateHandlerFactory) NewOpenerCreaterHandler() *LocalOpenCreateHandler {
	return NewLocalOpenCreateHandler().
		WithFileStor(h.fileStor).
		WithTransferRequestStor(h.transferRequestStor).
		WithKnownFileTracker(h.knownFilesTracker).
		WithFileHandleFactory(h.fileHandleFactory)
}

func (h *LocalOpenCreateHandlerFactory) WithFileHandleFactory(fileHandleFactory FileHandleFactory) *LocalOpenCreateHandlerFactory {
	h.fileHandleFactory = fileHandleFactory
	return h
}

type LocalOpenCreateHandler struct {
	knownFilesTracker   *KnownFilesTracker
	transferRequestStor stor.TransferRequestStor
	fileStor            stor.FileStor
	fileHandleFactory   FileHandleFactory
}

func NewLocalOpenCreateHandler() *LocalOpenCreateHandler {
	return &LocalOpenCreateHandler{}
}

func (h *LocalOpenCreateHandler) WithKnownFileTracker(knownFilesTracker *KnownFilesTracker) *LocalOpenCreateHandler {
	h.knownFilesTracker = knownFilesTracker
	return h
}

func (h *LocalOpenCreateHandler) WithTransferRequestStor(transferRequestStor stor.TransferRequestStor) *LocalOpenCreateHandler {
	h.transferRequestStor = transferRequestStor
	return h
}

func (h *LocalOpenCreateHandler) WithFileStor(fileStor stor.FileStor) *LocalOpenCreateHandler {
	h.fileStor = fileStor
	return h
}

func (h *LocalOpenCreateHandler) WithFileHandleFactory(fileHandleFactory FileHandleFactory) *LocalOpenCreateHandler {
	h.fileHandleFactory = fileHandleFactory
	return h
}

func (h *LocalOpenCreateHandler) Open(_ context.Context, path string, flags uint32) (fh fs.FileHandle, errno syscall.Errno) {
	var (
		err       error
		knownFile *mcmodel.File
	)

	projPath := projectpath.NewProjectPath(path)

	switch flags & syscall.O_ACCMODE {
	case syscall.O_RDONLY:
		knownFile = h.knownFilesTracker.GetFile(path)
	case syscall.O_WRONLY:
		knownFile = h.knownFilesTracker.GetFile(path)
		if knownFile == nil {
			knownFile, err = h.createNewMCFileVersion(projPath.ProjectPath)
			if err != nil {
				// TODO: What error should be returned?
				return nil, syscall.EIO
			}

			h.knownFilesTracker.Store(path, knownFile)
		}
		flags = flags &^ syscall.O_CREAT
		flags = flags &^ syscall.O_APPEND
	case syscall.O_RDWR:
		knownFile = knownFilesTracker.GetFile(path)
		if knownFile == nil {
			knownFile, err = h.createNewMCFileVersion(projPath.ProjectPath)
			if err != nil {
				// TODO: What error should be returned?
				return nil, syscall.EIO
			}
			knownFilesTracker.Store(path, knownFile)
		}
		flags = flags &^ syscall.O_CREAT
		flags = flags &^ syscall.O_APPEND
	default:
		return
	}

	filePath := knownFile.ToUnderlyingFilePath(mcfsRoot) //n.file.ToUnderlyingFilePath(mcfsRoot)
	//if knownFile != nil {
	//	filePath = knownFile.ToUnderlyingFilePath(mcfsRoot)
	//}
	fd, err := syscall.Open(filePath, int(flags), 0)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	fhandle := h.fileHandleFactory.NewFileHandle(fd, path, knownFile)
	return fhandle, fs.OK
}

func (h *LocalOpenCreateHandler) Create(_ context.Context, path string, flags, mode uint32) (fh fs.FileHandle, errno syscall.Errno) {
	projPath := projectpath.NewProjectPath(path)
	f, err := h.createNewMCFile(projPath.ProjectID, projPath.ProjectPath)
	if err != nil {
		slog.Error("Create - failed creating new file (%s): %s", path, err)
		return nil, syscall.EIO
	}

	h.knownFilesTracker.Store(path, f)

	flags = flags &^ syscall.O_APPEND
	fd, err := syscall.Open(f.ToUnderlyingFilePath(mcfsRoot), int(flags)|os.O_CREATE, mode)
	if err != nil {
		slog.Error("    Create - syscall.Open failed:", err)
		return nil, syscall.EIO
	}

	statInfo := syscall.Stat_t{}
	if err := syscall.Fstat(fd, &statInfo); err != nil {
		// TODO - Remove newly created file version in db
		_ = syscall.Close(fd)
		return nil, fs.ToErrno(err)
	}

	return h.fileHandleFactory.NewFileHandle(fd, path, f), fs.OK
}

func (h *LocalOpenCreateHandler) createNewMCFileVersion(fullPath string) (*mcmodel.File, error) {
	// First check if there is already a version of this file being written to for this upload context.
	existingFile := knownFilesTracker.GetFile(fullPath)
	if existingFile != nil {
		return existingFile, nil
	}

	var err error

	// There isn't an existing upload, so create a new one
	newFile := &mcmodel.File{
		ProjectID:   existingFile.ProjectID,
		Name:        existingFile.Name,
		DirectoryID: existingFile.DirectoryID,
		Size:        0,
		Checksum:    "",
		MimeType:    existingFile.MimeType,
		OwnerID:     existingFile.OwnerID,
		Current:     false,
	}

	newFile, err = transferRequestStore.CreateNewFile(newFile, existingFile.Directory, transferRequest)
	if err != nil {
		return nil, err
	}

	// Create the empty file for new version
	f, err := os.OpenFile(newFile.ToUnderlyingFilePath(mcfsRoot), os.O_RDWR|os.O_CREATE, 0755)

	if err != nil {
		slog.Error("os.OpenFile failed (%s): %s\n", newFile.ToUnderlyingFilePath(mcfsRoot), err)
		return nil, err
	}

	_ = f.Close()

	return newFile, nil
}

func (h *LocalOpenCreateHandler) createNewMCFile(projectID int, name string) (*mcmodel.File, error) {
	dir, err := getMCDir(projectID, "")
	if err != nil {
		return nil, err
	}

	file := &mcmodel.File{
		ProjectID:   projectID,
		Name:        name,
		DirectoryID: dir.ID,
		Size:        0,
		Checksum:    "",
		MimeType:    getMimeType(name),
		OwnerID:     transferRequest.OwnerID,
		Current:     false,
	}

	return transferRequestStore.CreateNewFile(file, dir, transferRequest)
}

func getMCDir(projectID int, path string) (*mcmodel.File, error) {
	//path := filepath.Join("/", n.Path(n.Root()), name)
	return fileStore.GetDirByPath(projectID, path)
}
