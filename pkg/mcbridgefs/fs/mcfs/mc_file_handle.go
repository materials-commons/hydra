package mcfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type MCFileHandle struct {
	*BaseFileHandle
	expectedOffset    int64
	Path              string
	File              *mcmodel.File
	activityCounter   *ActivityCounter
	mcfsapi           MCFSApi
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

func (h *MCFileHandle) WithActivityCounter(activityCounter *ActivityCounter) *MCFileHandle {
	h.activityCounter = activityCounter
	return h
}

func (h *MCFileHandle) WithMCFSApi(mcfsapi MCFSApi) *MCFileHandle {
	h.mcfsapi = mcfsapi
	return h
}

func (h *MCFileHandle) WithKnownFilesTracker(tracker *KnownFilesTracker) *MCFileHandle {
	h.knownFilesTracker = tracker
	return h
}

func (h *MCFileHandle) Write(_ context.Context, data []byte, off int64) (bytesWritten uint32, errno syscall.Errno) {
	h.Mu.Lock()
	fmt.Printf("MCFileHandle.Write %s:%d\n", string(data), off)
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

	if !flagSet(h.Flags, os.O_APPEND) {
		// If the O_APPEND flag is not set then we need to track
		// the offset. If it was set, then each write will automatically
		// be done to the end of the file.
		if !knownFile.hashInvalid {
			if h.expectedOffset != off {
				knownFile.hashInvalid = true
			}
		}
	}

	h.activityCounter.IncrementActivityCount()
	n, err := syscall.Pwrite(h.Fd, data, off)
	if err != nil {
		return uint32(n), fs.ToErrno(err)
	}

	h.expectedOffset = h.expectedOffset + int64(n)

	if !knownFile.hashInvalid {
		_, _ = io.Copy(knownFile.hasher, bytes.NewBuffer(data[:n]))
	}

	return uint32(n), fs.OK
}

func (h *MCFileHandle) Read(_ context.Context, buf []byte, off int64) (res fuse.ReadResult, errno syscall.Errno) {
	h.Mu.Lock()
	fmt.Println("MCFileHandle.Read")
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
			errno = syscall.EBADF
		}
		h.Mu.Unlock()
	}()

	if h.Fd == -1 {
		fmt.Printf("h.Fd == -1 for %s\n", h.Path)
		return syscall.EBADF
	}

	err := syscall.Close(h.Fd)
	h.Fd = -1
	if err != nil {
		fmt.Printf("MCFileHandle.Release syscall.Close failed %s: %s\n", h.Path, err)
		return fs.ToErrno(err)
	}

	// if file was opened readonly then there is no need to do any updates to the database
	omode := h.Flags & syscall.O_ACCMODE
	if omode == syscall.O_RDONLY {
		return fs.OK
	}

	var (
		size  uint64 = 0
		attrs fuse.AttrOut
	)

	if err := h.BaseFileHandle.getattr(ctx, &attrs); err == fs.OK {
		size = attrs.Size
	}

	return fs.ToErrno(h.mcfsapi.Release(h.Path, size))
}

func (h *MCFileHandle) Setattr(_ context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) (errno syscall.Errno) {
	slog.Debug("MCFileHandle.Setattr")
	fmt.Println("MCFileHandle.Setattr")
	h.Mu.Lock()
	defer func() {
		if r := recover(); r != nil {
			errno = syscall.EIO
		}
		h.Mu.Unlock()
	}()

	if sz, ok := in.GetSize(); ok {
		if err := syscall.Ftruncate(h.Fd, int64(sz)); err != nil {
			return fs.ToErrno(err)
		}

		st := syscall.Stat_t{}
		if err := syscall.Fstat(h.Fd, &st); err != nil {
			return fs.ToErrno(err)
		}
		out.FromStat(&st)
	}

	return fs.OK
}
