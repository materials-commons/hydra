package mcfs

import (
	"context"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
)

type OpenerCreater interface {
	Open(ctx context.Context, path string, flags uint32) (fh fs.FileHandle, errno syscall.Errno)
	Create(ctx context.Context, path string, flags, mode uint32) (fh fs.FileHandle, errno syscall.Errno)
}

type LocalOpenCreateHandler struct {
}

//func (h *LocalOpenCreateHandler) Open(_ context.Context, path string, flags uint32) (fh fs.FileHandle, errno syscall.Errno) {
//	var (
//		err     error
//		newFile *mcmodel.File
//	)
//
//	switch flags & syscall.O_ACCMODE {
//	case syscall.O_RDONLY:
//		newFile = getFromOpenedFiles(path)
//	case syscall.O_WRONLY:
//		newFile = getFromOpenedFiles(path)
//		if newFile == nil {
//			newFile, err = n.createNewMCFileVersion()
//			if err != nil {
//				// TODO: What error should be returned?
//				return nil, syscall.EIO
//			}
//
//			openedFilesTracker.Store(path, newFile)
//		}
//		flags = flags &^ syscall.O_CREAT
//		flags = flags &^ syscall.O_APPEND
//	case syscall.O_RDWR:
//		newFile = getFromOpenedFiles(path)
//		if newFile == nil {
//			newFile, err = n.createNewMCFileVersion()
//			if err != nil {
//				// TODO: What error should be returned?
//				return nil, syscall.EIO
//			}
//			openedFilesTracker.Store(path, newFile)
//		}
//		flags = flags &^ syscall.O_CREAT
//		flags = flags &^ syscall.O_APPEND
//	default:
//		return
//	}
//
//	filePath := n.file.ToUnderlyingFilePath(mcfsRoot)
//	if newFile != nil {
//		filePath = newFile.ToUnderlyingFilePath(mcfsRoot)
//	}
//	fd, err := syscall.Open(filePath, int(flags), 0)
//	if err != nil {
//		return nil, fs.ToErrno(err)
//	}
//
//	fhandle := NewFileHandle(fd, flags, path)
//	return fhandle, fs.OK
//}
//
//func (h *LocalOpenCreateHandler) Create(_ context.Context, path string, flags, mode uint32) (fh fs.FileHandle, errno syscall.Errno) {
//	f, err := n.createNewMCFile(name)
//	if err != nil {
//		slog.Error("Create - failed creating new file (%s): %s", name, err)
//		return nil, syscall.EIO
//	}
//
//	openedFilesTracker.Store(path, f)
//
//	flags = flags &^ syscall.O_APPEND
//	fd, err := syscall.Open(f.ToUnderlyingFilePath(mcfsRoot), int(flags)|os.O_CREATE, mode)
//	if err != nil {
//		slog.Error("    Create - syscall.Open failed:", err)
//		return nil, syscall.EIO
//	}
//
//	statInfo := syscall.Stat_t{}
//	if err := syscall.Fstat(fd, &statInfo); err != nil {
//		// TODO - Remove newly created file version in db
//		_ = syscall.Close(fd)
//		return nil, fs.ToErrno(err)
//	}
//
//	return nil, fs.OK
//}
//
//func createNewMCFileVersion() (*mcmodel.File, error) {
//	// First check if there is already a version of this file being written to for this upload context.
//	existing := getFromOpenedFiles(filepath.Join("/", n.Path(n.Root()), n.file.Name))
//	if existing != nil {
//		return existing, nil
//	}
//
//	var err error
//
//	// There isn't an existing upload, so create a new one
//	newFile := &mcmodel.File{
//		ProjectID:   n.file.ProjectID,
//		Name:        n.file.Name,
//		DirectoryID: n.file.DirectoryID,
//		Size:        0,
//		Checksum:    "",
//		MimeType:    n.file.MimeType,
//		OwnerID:     n.file.OwnerID,
//		Current:     false,
//	}
//
//	newFile, err = transferRequestStore.CreateNewFile(newFile, n.file.Directory, transferRequest)
//	if err != nil {
//		return nil, err
//	}
//
//	// Create the empty file for new version
//	f, err := os.OpenFile(newFile.ToUnderlyingFilePath(mcfsRoot), os.O_RDWR|os.O_CREATE, 0755)
//
//	if err != nil {
//		slog.Error("os.OpenFile failed (%s): %s\n", newFile.ToUnderlyingFilePath(mcfsRoot), err)
//		return nil, err
//	}
//	defer func() { _ = f.Close() }()
//
//	return newFile, nil
//}
//
//func createNewMCFile(name string) (*mcmodel.File, error) {
//	dir, err := n.getMCDir("")
//	if err != nil {
//		return nil, err
//	}
//
//	file := &mcmodel.File{
//		ProjectID:   transferRequest.ProjectID,
//		Name:        name,
//		DirectoryID: dir.ID,
//		Size:        0,
//		Checksum:    "",
//		MimeType:    getMimeType(name),
//		OwnerID:     transferRequest.OwnerID,
//		Current:     false,
//	}
//
//	return transferRequestStore.CreateNewFile(file, dir, transferRequest)
//}
//
//func getMCDir(name string) (*mcmodel.File, error) {
//	path := filepath.Join("/", n.Path(n.Root()), name)
//	return fileStore.GetDirByPath(transferRequest.ProjectID, path)
//}
