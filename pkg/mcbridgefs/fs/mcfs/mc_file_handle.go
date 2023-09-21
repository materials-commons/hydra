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
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type MCFileHandle struct {
	*BaseFileHandle
	Path              string
	File              *mcmodel.File
	activityCounter   ActivityCounter
	mcapi             *MCApi
	knownFilesTracker *KnownFilesTracker
}

var _ = (fs.FileHandle)((*MCFileHandle)(nil))
var _ = (fs.FileWriter)((*MCFileHandle)(nil))
var _ = (fs.FileReader)((*MCFileHandle)(nil))
var _ = (fs.FileFlusher)((*MCFileHandle)(nil))
var _ = (fs.FileSetattrer)((*MCFileHandle)(nil))

func NewMCFileHandle(fd, flags int) *MCFileHandle {
	return &MCFileHandle{
		BaseFileHandle: NewBaseFileHandle(fd, flags).(*BaseFileHandle),
	}
}

func (h *MCFileHandle) WithPath(path string) *MCFileHandle {
	h.Path = path
	return h
}

func (h *MCFileHandle) WithFile(f *mcmodel.File) *MCFileHandle {
	h.File = f
	return h
}

func (h *MCFileHandle) WithActivityCounter(activityCounter ActivityCounter) *MCFileHandle {
	h.activityCounter = activityCounter
	return h
}

func (h *MCFileHandle) WithMCApi(mcapi *MCApi) *MCFileHandle {
	h.mcapi = mcapi
	return h
}

func (h *MCFileHandle) WithKnownFilesTracker(tracker *KnownFilesTracker) *MCFileHandle {
	h.knownFilesTracker = tracker
	return h
}

func (h *MCFileHandle) Write(_ context.Context, data []byte, off int64) (bytesWritten uint32, errno syscall.Errno) {
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
		slog.Error("Unknown file in MCFileHandle", "path", h.Path)
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

func (h *MCFileHandle) Read(_ context.Context, buf []byte, off int64) (res fuse.ReadResult, errno syscall.Errno) {
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

func (h *MCFileHandle) Flush(_ context.Context) syscall.Errno {
	return fs.OK
}

func (h *MCFileHandle) Release(ctx context.Context) (errno syscall.Errno) {
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
		size  uint64 = 0
		attrs fuse.AttrOut
	)

	if err := h.BaseFileHandle.getattr(ctx, &attrs); err == fs.OK {
		size = attrs.Size
	}

	return fs.ToErrno(h.mcapi.Release(h.Path, size))
}

func (h *MCFileHandle) Setattr(_ context.Context, in *fuse.SetAttrIn, _ *fuse.AttrOut) (errno syscall.Errno) {
	fmt.Println("MCFileHandle.Setattr called")
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
