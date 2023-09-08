package mcfs

import (
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/materials-commons/hydra/pkg/mcbridgefs/fs/mcfs/projectpath"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type FileHandleFactory interface {
	NewFileHandle(fd int, path string, file *mcmodel.File) fs.FileHandle
}

type LocalFileHandlerFactory struct {
	conversionStor         stor.ConversionStor
	transferRequestStor    stor.TransferRequestStor
	activityCounterFactory *PathBasedActivityCounterFactory
}

func NewLocalFileHandlerFactory(conversionStor stor.ConversionStor, transferRequestStor stor.TransferRequestStor) *LocalFileHandlerFactory {
	return &LocalFileHandlerFactory{
		conversionStor:         conversionStor,
		transferRequestStor:    transferRequestStor,
		activityCounterFactory: NewPathBasedActivityCounterFactory(),
	}
}

func (f *LocalFileHandlerFactory) NewFileHandle(fd int, path string, file *mcmodel.File) *LocalMonitoredFileHandle {
	projPath := projectpath.NewProjectPath(path)
	activityCounter := f.activityCounterFactory.GetOrCreateActivityCounter(projPath.TransferBase)
	return NewLocalMonitoredFileHandle(fd).
		WithPath(path).
		WithFile(file).
		WithActivityCounter(activityCounter).
		WithConversionStor(f.conversionStor).
		WithTransferRequestStor(f.transferRequestStor)
}
