package mcfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/materials-commons/hydra/pkg/mcbridgefs/fs/mcfs/projectpath"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type LocalMonitoredFileHandle struct {
	*BaseLocalFileHandle
	Path                string
	File                *mcmodel.File
	activityCounter     ActivityCounter
	transferRequestStor stor.TransferRequestStor
	conversionStor      stor.ConversionStor
	knownFilesTracker   *KnownFilesTracker
}

var _ = (fs.FileHandle)((*LocalMonitoredFileHandle)(nil))
var _ = (fs.FileWriter)((*LocalMonitoredFileHandle)(nil))
var _ = (fs.FileReader)((*LocalMonitoredFileHandle)(nil))
var _ = (fs.FileFlusher)((*LocalMonitoredFileHandle)(nil))
var _ = (fs.FileSetattrer)((*LocalMonitoredFileHandle)(nil))

func NewLocalMonitoredFileHandle(fd, flags int) *LocalMonitoredFileHandle {
	return &LocalMonitoredFileHandle{
		BaseLocalFileHandle: NewBaseLocalFileHandle(fd, flags).(*BaseLocalFileHandle),
	}
}

func (h *LocalMonitoredFileHandle) WithPath(path string) *LocalMonitoredFileHandle {
	h.Path = path
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

func (h *LocalMonitoredFileHandle) WithKnownFilesTracker(tracker *KnownFilesTracker) *LocalMonitoredFileHandle {
	h.knownFilesTracker = tracker
	return h
}

func (h *LocalMonitoredFileHandle) Write(_ context.Context, data []byte, off int64) (bytesWritten uint32, errno syscall.Errno) {
	h.Mu.Lock()
	defer func() {
		if r := recover(); r != nil {
			bytesWritten = 0
			errno = syscall.EIO
		}
		h.Mu.Unlock()
	}()

	knownFile := h.knownFilesTracker.Get(h.Path)
	if knownFile == nil {
		slog.Error("Unknown file in LocalMonitoredFileHandle", "path", h.Path)
		return 0, syscall.EIO
	}

	h.activityCounter.IncrementActivityCount()
	n, err := syscall.Pwrite(h.Fd, data, off)
	if err != nil {
		return uint32(n), fs.ToErrno(err)
	}

	_, _ = io.Copy(knownFile.hasher, bytes.NewBuffer(data[:n]))

	return uint32(n), fs.OK
}

func (h *LocalMonitoredFileHandle) Read(_ context.Context, buf []byte, off int64) (res fuse.ReadResult, errno syscall.Errno) {
	h.Mu.Lock()
	defer func() {
		if r := recover(); r != nil {
			res = nil
			errno = syscall.EIO
		}
		h.Mu.Unlock()
	}()

	h.activityCounter.IncrementActivityCount()

	r := fuse.ReadResultFd(uintptr(h.Fd), off, len(buf))
	return r, fs.OK
}

func (h *LocalMonitoredFileHandle) Flush(_ context.Context) syscall.Errno {
	return fs.OK
}

func (h *LocalMonitoredFileHandle) Release(ctx context.Context) (errno syscall.Errno) {
	h.Mu.Lock()
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Relase panicked")
			errno = syscall.EBADF
		}
		h.Mu.Unlock()
	}()

	if h.Fd == -1 {
		fmt.Println("Release h.Fd == -1")
		return syscall.EBADF
	}

	err := syscall.Close(h.Fd)
	fmt.Println("syscall.Close err =", err)
	h.Fd = -1
	if err != nil {
		fmt.Println("syscall.Close gave error")
		return fs.ToErrno(err)
	}

	// if file was opened readonly then there is no need to do any updates to the database
	omode := h.Flags & syscall.O_ACCMODE
	if omode == syscall.O_RDONLY {
		fmt.Println("is readonly")
		return fs.OK
	}

	var (
		size     uint64 = 0
		attrs    fuse.AttrOut
		checksum string
	)

	if err := h.BaseLocalFileHandle.getattr(ctx, &attrs); err == fs.OK {
		size = attrs.Size
	}

	knownFile := h.knownFilesTracker.Get(h.Path)
	if knownFile == nil {
		fmt.Println("knownFilesTracker didn't find file")
		return syscall.ENOENT
	}

	projPath := projectpath.NewProjectPath(h.Path)

	checksum = fmt.Sprintf("%x", knownFile.hasher.Sum(nil))

	errno = fs.ToErrno(h.transferRequestStor.MarkFileReleased(h.File, checksum, projPath.ProjectID, int64(size)))

	// Add to convertible list after marking as released to prevent the condition where the
	// file hasn't been released but is picked up for conversion. This is a very unlikely
	// case, but easy to prevent by releasing then adding to conversions list.
	if h.File.IsConvertible() {
		if _, err := h.conversionStor.AddFileToConvert(h.File); err != nil {
			slog.Error("Failed adding file to conversion", "file.ID", h.File.ID)
		}
	}

	fmt.Println("Got through release errno = ", errno)
	return errno
}

func (h *LocalMonitoredFileHandle) Setattr(_ context.Context, in *fuse.SetAttrIn, _ *fuse.AttrOut) (errno syscall.Errno) {
	fmt.Println("LocalMonitoredFileHandle.Setattr called")
	h.Mu.Lock()
	defer func() {
		if r := recover(); r != nil {
			errno = syscall.EIO
		}
		h.Mu.Unlock()
	}()

	if sz, ok := in.GetSize(); ok {
		return fs.ToErrno(syscall.Ftruncate(h.Fd, int64(sz)))
	}

	return fs.OK
}
