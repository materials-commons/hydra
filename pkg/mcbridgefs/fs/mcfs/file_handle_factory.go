package mcfs

import (
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/materials-commons/hydra/pkg/mcbridgefs/fs/mcfs/projectpath"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type FileHandleFactory interface {
	NewFileHandle(fd, flags int, path string, file *mcmodel.File) fs.FileHandle
}

type MCFileHandlerFactory struct {
	mcapi                  *LocalMCFSApi
	activityCounterFactory *ActivityCounterMonitor
	knownFilesTracker      *KnownFilesTracker
}

func NewMCFileHandlerFactory(mcapi *LocalMCFSApi, knownFilesTracker *KnownFilesTracker, inactivity time.Duration) *MCFileHandlerFactory {
	return &MCFileHandlerFactory{
		mcapi:                  mcapi,
		activityCounterFactory: NewActivityCounterMonitor(inactivity),
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
		WithMCFSApi(f.mcapi)
}
