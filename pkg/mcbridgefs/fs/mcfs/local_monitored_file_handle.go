package mcfs

import (
	"bytes"
	"context"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type LocalMonitoredFileHandle struct {
	*BaseLocalFileHandle
	hasher              hash.Hash
	Checksum            string
	File                *mcmodel.File
	activityCounter     ActivityCounter
	transferRequestStor stor.TransferRequestStor
	conversionStor      stor.ConversionStor
}

type LocalMonitoredFileHandleOptionFunc func(handle *LocalMonitoredFileHandle)

var _ = (fs.FileHandle)((*LocalMonitoredFileHandle)(nil))
var _ = (fs.FileWriter)((*LocalMonitoredFileHandle)(nil))
var _ = (fs.FileReader)((*LocalMonitoredFileHandle)(nil))
var _ = (fs.FileFlusher)((*LocalMonitoredFileHandle)(nil))

func NewLocalMonitoredFileHandle(fd int) *LocalMonitoredFileHandle {
	return &LocalMonitoredFileHandle{
		BaseLocalFileHandle: NewBaseLocalFileHandle(fd).(*BaseLocalFileHandle),
	}
}

func (h *LocalMonitoredFileHandle) WithHasher(hasher hash.Hash) *LocalMonitoredFileHandle {
	h.hasher = hasher
	return h
}

func (h *LocalMonitoredFileHandle) WithFile(f *mcmodel.File) *LocalMonitoredFileHandle {
	h.File = f
	return h
}

func (h *LocalMonitoredFileHandle) WithActivityCounter(activityCounter ActivityCounter) *LocalMonitoredFileHandle {
	h.activityCounter = activityCounter
	return h
}

func (h *LocalMonitoredFileHandle) WithTransferRequestStor(s stor.TransferRequestStor) *LocalMonitoredFileHandle {
	h.transferRequestStor = s
	return h
}

func (h *LocalMonitoredFileHandle) WithConversionStor(s stor.ConversionStor) *LocalMonitoredFileHandle {
	h.conversionStor = s
	return h
}

func (h *LocalMonitoredFileHandle) Write(_ context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	h.Mu.Lock()
	defer h.Mu.Unlock()

	h.activityCounter.IncrementActivityCount()
	n, err := syscall.Pwrite(h.Fd, data, off)
	if err != nil {
		return uint32(n), fs.ToErrno(err)
	}

	_, _ = io.Copy(h.hasher, bytes.NewBuffer(data[:n]))

	return uint32(n), fs.OK
}

func (h *LocalMonitoredFileHandle) Read(_ context.Context, buf []byte, off int64) (res fuse.ReadResult, errno syscall.Errno) {
	h.Mu.Lock()
	defer h.Mu.Unlock()

	h.activityCounter.IncrementActivityCount()

	r := fuse.ReadResultFd(uintptr(h.Fd), off, len(buf))
	return r, fs.OK
}

func (h *LocalMonitoredFileHandle) Flush(_ context.Context) syscall.Errno {
	return fs.OK
}

func (h *LocalMonitoredFileHandle) Release(ctx context.Context) syscall.Errno {
	if err := h.BaseLocalFileHandle.Release(ctx); err != fs.OK {
		return err
	}

	var (
		size     uint64 = 0
		attrs    fuse.AttrOut
		checksum string
	)

	if err := h.BaseLocalFileHandle.Getattr(ctx, &attrs); err == fs.OK {
		size = attrs.Size
	}

	checksum = fmt.Sprintf("%x", h.hasher.Sum(nil))

	errno := fs.ToErrno(transferRequestStore.MarkFileReleased(h.File, checksum, transferRequest.ProjectID, int64(size)))

	// Add to convertible list after marking as released to prevent the condition where the
	// file hasn't been released but is picked up for conversion. This is a very unlikely
	// case, but easy to prevent by releasing then adding to conversions list.
	if h.File.IsConvertible() {
		if _, err := conversionStore.AddFileToConvert(h.File); err != nil {
			slog.Error("Failed adding file to conversion: %d", h.File.ID)
		}
	}

	return errno

}
