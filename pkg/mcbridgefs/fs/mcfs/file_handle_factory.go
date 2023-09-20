package mcfs

import (
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/materials-commons/hydra/pkg/mcbridgefs/fs/mcfs/projectpath"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type FileHandleFactory interface {
	NewFileHandle(fd, flags int, path string, file *mcmodel.File) fs.FileHandle
}

type LocalFileHandlerFactory struct {
	conversionStor         stor.ConversionStor
	transferRequestStor    stor.TransferRequestStor
	activityCounterFactory *PathBasedActivityCounterFactory
	knownFilesTracker      *KnownFilesTracker
}

func NewLocalFileHandlerFactory(conversionStor stor.ConversionStor, transferRequestStor stor.TransferRequestStor, knownFilesTracker *KnownFilesTracker) *LocalFileHandlerFactory {
	return &LocalFileHandlerFactory{
		conversionStor:         conversionStor,
		transferRequestStor:    transferRequestStor,
		activityCounterFactory: NewPathBasedActivityCounterFactory(),
		knownFilesTracker:      knownFilesTracker,
	}
}

func (f *LocalFileHandlerFactory) NewFileHandle(fd, flags int, path string, file *mcmodel.File) fs.FileHandle {
	projPath := projectpath.NewProjectPath(path)
	activityCounter := f.activityCounterFactory.GetOrCreateActivityCounter(projPath.TransferBase)
	return NewLocalMonitoredFileHandle(fd, flags).
		WithPath(path).
		WithFile(file).
		WithActivityCounter(activityCounter).
		WithKnownFilesTracker(f.knownFilesTracker).
		WithConversionStor(f.conversionStor).
		WithTransferRequestStor(f.transferRequestStor)
}
