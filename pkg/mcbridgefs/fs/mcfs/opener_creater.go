package mcfs

import (
	"context"
	"log/slog"
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/materials-commons/hydra/pkg/mcbridgefs/fs/mcfs/projectpath"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type OpenerCreater interface {
	Open(ctx context.Context, path string, flags uint32) (fh fs.FileHandle, errno syscall.Errno)
	Create(ctx context.Context, path string, flags, mode uint32) (fh fs.FileHandle, errno syscall.Errno)
}

type LocalOpenCreateHandler struct {
}

func (h *LocalOpenCreateHandler) Open(_ context.Context, path string, flags uint32) (fh fs.FileHandle, errno syscall.Errno) {
	var (
		err       error
		knownFile *mcmodel.File
	)

	projPath := projectpath.NewProjectPath(path)

	switch flags & syscall.O_ACCMODE {
	case syscall.O_RDONLY:
		knownFile = knownFilesTracker.Get(path)
	case syscall.O_WRONLY:
		knownFile = knownFilesTracker.Get(path)
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
	case syscall.O_RDWR:
		knownFile = knownFilesTracker.Get(path)
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

	fhandle := NewFileHandle(fd, flags, path)
	return fhandle, fs.OK
}

func (h *LocalOpenCreateHandler) Create(_ context.Context, path string, flags, mode uint32) (fh fs.FileHandle, errno syscall.Errno) {
	projPath := projectpath.NewProjectPath(path)
	f, err := h.createNewMCFile(projPath.ProjectID, projPath.ProjectPath)
	if err != nil {
		slog.Error("Create - failed creating new file (%s): %s", path, err)
		return nil, syscall.EIO
	}

	knownFilesTracker.Store(path, f)

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

	return nil, fs.OK // return file handle here not nil
}

func (h *LocalOpenCreateHandler) createNewMCFileVersion(fullPath string) (*mcmodel.File, error) {
	// First check if there is already a version of this file being written to for this upload context.
	existingFile := knownFilesTracker.Get(fullPath)
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
