package mcfs

import (
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/materials-commons/hydra/pkg/mcbridgefs/fs/mcfs/projectpath"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type FileHandleFactory interface {
	NewFileHandle(fd, flags int, path string, file *mcmodel.File) fs.FileHandle
}

type MCFileHandlerFactory struct {
	mcapi                  *MCApi
	activityCounterFactory *PathBasedActivityCounterFactory
	knownFilesTracker      *KnownFilesTracker
}

func NewMCFileHandlerFactory(mcapi *MCApi, knownFilesTracker *KnownFilesTracker) *MCFileHandlerFactory {
	return &MCFileHandlerFactory{
		mcapi:                  mcapi,
		activityCounterFactory: NewPathBasedActivityCounterFactory(),
		knownFilesTracker:      knownFilesTracker,
	}
}

func (f *MCFileHandlerFactory) NewFileHandle(fd, flags int, path string, file *mcmodel.File) fs.FileHandle {
	projPath := projectpath.NewProjectPath(path)
	activityCounter := f.activityCounterFactory.GetOrCreateActivityCounter(projPath.TransferBase)
	return NewMCFileHandle(fd, flags).
		WithPath(path).
		WithFile(file).
		WithActivityCounter(activityCounter).
		WithKnownFilesTracker(f.knownFilesTracker).
		WithMCApi(f.mcapi)
}
