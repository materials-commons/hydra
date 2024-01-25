// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mcfs

import (
	"context"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/materials-commons/hydra/pkg/clog"

	"github.com/hanwen/go-fuse/v2/fuse"
	"golang.org/x/sys/unix"
)

// BaseFileHandle implements the basic operations to a file. It is heavily copied from
// github.com/hanwen/go-fuse/fs/files.go. It implements an interface to an existing
// file in a linux file system.
type BaseFileHandle struct {
	// Mu is used to lock operations on the file
	Mu sync.Mutex

	// The file descriptor to the file the BaseFileHandle is connected to
	Fd int

	// The flags used to open the file.
	Flags int
}

// NewBaseFileHandle creates a FileHandle out of a file descriptor. All
// operations are implemented. When using the Fd from a *os.File, call
// syscall.Dup() on the Fd, to avoid os.File's finalizer from closing
// the file descriptor.
func NewBaseFileHandle(fd, flags int) fs.FileHandle {
	return &BaseFileHandle{Fd: fd, Flags: flags}
}

var _ = (fs.FileHandle)((*BaseFileHandle)(nil))
var _ = (fs.FileReleaser)((*BaseFileHandle)(nil))
var _ = (fs.FileGetattrer)((*BaseFileHandle)(nil))
var _ = (fs.FileReader)((*BaseFileHandle)(nil))
var _ = (fs.FileWriter)((*BaseFileHandle)(nil))
var _ = (fs.FileGetlker)((*BaseFileHandle)(nil))
var _ = (fs.FileSetlker)((*BaseFileHandle)(nil))
var _ = (fs.FileSetlkwer)((*BaseFileHandle)(nil))
var _ = (fs.FileLseeker)((*BaseFileHandle)(nil))
var _ = (fs.FileFlusher)((*BaseFileHandle)(nil))
var _ = (fs.FileFsyncer)((*BaseFileHandle)(nil))
var _ = (fs.FileSetattrer)((*BaseFileHandle)(nil))
var _ = (fs.FileAllocater)((*BaseFileHandle)(nil))

// Read reads a from the file descriptor.
func (f *BaseFileHandle) Read(ctx context.Context, buf []byte, off int64) (res fuse.ReadResult, errno syscall.Errno) {
	//clog.Global().Debug("BaseFileHandle Read")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	r := fuse.ReadResultFd(uintptr(f.Fd), off, len(buf))
	return r, fs.OK
}

// Write writes to the file descriptor.
func (f *BaseFileHandle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	//clog.Global().Debug("BaseFileHandle Write")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	n, err := syscall.Pwrite(f.Fd, data, off)
	return uint32(n), fs.ToErrno(err)
}

// Release handles the close operation.
func (f *BaseFileHandle) Release(ctx context.Context) syscall.Errno {
	//clog.Global().Debug("BaseFileHandle Release")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.Fd != -1 {
		err := syscall.Close(f.Fd)
		f.Fd = -1
		return fs.ToErrno(err)
	}
	return syscall.EBADF
}

// Flush handles flushing the file buffer.
func (f *BaseFileHandle) Flush(ctx context.Context) syscall.Errno {
	//clog.Global().Debug("BaseFileHandle Flush")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	// Since Flush() may be called for each dup'd Fd, we don't
	// want to really close the file, we just want to flush. This
	// is achieved by closing a dup'd Fd.
	newFd, err := syscall.Dup(f.Fd)

	if err != nil {
		return fs.ToErrno(err)
	}
	err = syscall.Close(newFd)
	return fs.ToErrno(err)
}

// Fsync handles the fsync call to flush to disk.
func (f *BaseFileHandle) Fsync(ctx context.Context, flags uint32) (errno syscall.Errno) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	r := fs.ToErrno(syscall.Fsync(f.Fd))

	return r
}

const (
	_OFD_GETLK  = 36
	_OFD_SETLK  = 37
	_OFD_SETLKW = 38
)

// Getlk implements getting file lock status
func (f *BaseFileHandle) Getlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) (errno syscall.Errno) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	flk := syscall.Flock_t{}
	lk.ToFlockT(&flk)
	errno = fs.ToErrno(syscall.FcntlFlock(uintptr(f.Fd), _OFD_GETLK, &flk))
	out.FromFlockT(&flk)
	return
}

// Setlk implements file locking
func (f *BaseFileHandle) Setlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32) (errno syscall.Errno) {
	return f.setLock(ctx, owner, lk, flags, false)
}

// Setlkw implements file locking with waiting
func (f *BaseFileHandle) Setlkw(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32) (errno syscall.Errno) {
	return f.setLock(ctx, owner, lk, flags, true)
}

// setLock is the actual implementation of file locking.
func (f *BaseFileHandle) setLock(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32, blocking bool) (errno syscall.Errno) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if (flags & fuse.FUSE_LK_FLOCK) != 0 {
		var op int
		switch lk.Typ {
		case syscall.F_RDLCK:
			op = syscall.LOCK_SH
		case syscall.F_WRLCK:
			op = syscall.LOCK_EX
		case syscall.F_UNLCK:
			op = syscall.LOCK_UN
		default:
			return syscall.EINVAL
		}
		if !blocking {
			op |= syscall.LOCK_NB
		}
		return fs.ToErrno(syscall.Flock(f.Fd, op))
	} else {
		flk := syscall.Flock_t{}
		lk.ToFlockT(&flk)
		var op int
		if blocking {
			op = _OFD_SETLKW
		} else {
			op = _OFD_SETLK
		}
		return fs.ToErrno(syscall.FcntlFlock(uintptr(f.Fd), op, &flk))
	}
}

// Setattr implements the FUSE Setattr call for setting attributes on a file
func (f *BaseFileHandle) Setattr(ctx context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	clog.Global().Debug("BaseFileHandle.Setattr")
	if errno := f.setAttr(ctx, in); errno != 0 {
		return errno
	}

	return f.Getattr(ctx, out)
}

// setAttr sets the actual file attributes, it doesn't grab the mutex
func (f *BaseFileHandle) setAttr(ctx context.Context, in *fuse.SetAttrIn) syscall.Errno {
	clog.Global().Debug("BaseFileHandle.setAttr")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	var errno syscall.Errno
	if mode, ok := in.GetMode(); ok {
		errno = fs.ToErrno(syscall.Fchmod(f.Fd, mode))
		if errno != 0 {
			return errno
		}
	}

	uid32, uOk := in.GetUID()
	gid32, gOk := in.GetGID()
	if uOk || gOk {
		uid := -1
		gid := -1

		if uOk {
			uid = int(uid32)
		}
		if gOk {
			gid = int(gid32)
		}
		errno = fs.ToErrno(syscall.Fchown(f.Fd, uid, gid))
		if errno != 0 {
			return errno
		}
	}

	mtime, mok := in.GetMTime()
	atime, aok := in.GetATime()

	if mok || aok {
		ap := &atime
		mp := &mtime
		if !aok {
			ap = nil
		}
		if !mok {
			mp = nil
		}
		errno = f.utimens(ap, mp)
		if errno != 0 {
			return errno
		}
	}

	if sz, ok := in.GetSize(); ok {
		errno = fs.ToErrno(syscall.Ftruncate(f.Fd, int64(sz)))
		if errno != 0 {
			return errno
		}
	}
	return fs.OK
}

// Getattr implements getting attributes about a file (different from stat)
func (f *BaseFileHandle) Getattr(ctx context.Context, a *fuse.AttrOut) syscall.Errno {
	clog.Global().Debug("BaseFileHandle.Getattr")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return f.getattr(ctx, a)
}

// getattr gets the file attributes, no mutex is used
func (f *BaseFileHandle) getattr(_ context.Context, a *fuse.AttrOut) syscall.Errno {
	clog.Global().Debug("  BaseFileHandle.getattr")
	st := syscall.Stat_t{}
	if err := syscall.Fstat(f.Fd, &st); err != nil {
		return fs.ToErrno(err)
	}

	a.FromStat(&st)

	return fs.OK
}

// Lseek implements file seeking
func (f *BaseFileHandle) Lseek(ctx context.Context, off uint64, whence uint32) (uint64, syscall.Errno) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	clog.Global().Debug("BaseFileHandle.Lseek")
	n, err := unix.Seek(f.Fd, int64(off), int(whence))
	return uint64(n), fs.ToErrno(err)
}
