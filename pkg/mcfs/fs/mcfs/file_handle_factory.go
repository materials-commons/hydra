package mcfs

import (
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/fsstate"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/mcpath"
)

// FileHandleFactory is an interface that wraps the method for getting a new file handle. This allows
// for file handles implementing different feature sets. A factory is used to create a file handle
// for the case where elements of the file handle need to share state, such as a common interface
// to the database.
type FileHandleFactory interface {
	NewFileHandle(fd, flags int, path string, file *mcmodel.File) fs.FileHandle
}

// MCFileHandlerFactory creates new instances of MCFileHandle. The shared state is the MCFSApi,
// an activity counter and a tracker for files that are or were opened.
type MCFileHandlerFactory struct {
	mcfsapi                MCFSApi
	activityCounterMonitor *fsstate.ActivityTracker
	transferStateTracker   *fsstate.TransferStateTracker
	pathParser             mcpath.Parser
}

// NewMCFileHandlerFactory creates a new MCFileHandlerFactory.
func NewMCFileHandlerFactory(mcfsapi MCFSApi, transferStateTracker *fsstate.TransferStateTracker, pathParser mcpath.Parser,
	activityCounterMonitor *fsstate.ActivityTracker) *MCFileHandlerFactory {
	return &MCFileHandlerFactory{
		mcfsapi:                mcfsapi,
		activityCounterMonitor: activityCounterMonitor,
		transferStateTracker:   transferStateTracker,
		pathParser:             pathParser,
	}
}

// NewFileHandle creates a new MCFileHandle. Handles created this way will share the activity counter,
// known files tracker and MCFSApi.
func (f *MCFileHandlerFactory) NewFileHandle(fd, flags int, path string, file *mcmodel.File) fs.FileHandle {
	p, _ := f.pathParser.Parse(path)
	activityCounter := f.activityCounterMonitor.GetOrCreateActivityCounter(p.TransferKey())
	return NewMCFileHandle(fd, flags).
		WithPathParser(f.pathParser).
		WithPath(path).
		WithFile(file).
		WithActivityCounter(activityCounter).
		WithTransferStateTracker(f.transferStateTracker).
		WithMCFSApi(f.mcfsapi)

}
