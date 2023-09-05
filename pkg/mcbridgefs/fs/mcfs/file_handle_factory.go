package mcfs

import (
	"crypto/md5"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type FileHandleFactoryer interface {
	NewFileHandle(fd int, file *mcmodel.File) fs.FileHandle
}

type MCFileHandlerFactory struct {
	conversionStor      stor.ConversionStor
	transferRequestStor stor.TransferRequestStor
}

func NewMCFileHandlerFactory(conversionStor stor.ConversionStor, transferRequestStor stor.TransferRequestStor) *MCFileHandlerFactory {
	return &MCFileHandlerFactory{
		conversionStor:      conversionStor,
		transferRequestStor: transferRequestStor,
	}
}

func (f *MCFileHandlerFactory) NewFileHandle(fd int, file *mcmodel.File) *LocalMonitoredFileHandle {
	return NewLocalMonitoredFileHandle(fd).
		WithFile(file).
		WithHasher(md5.New()).
		WithActivityCounter(NewFSActivityCounter()).
		WithConversionStor(f.conversionStor).
		WithTransferRequestStor(f.transferRequestStor)
}
