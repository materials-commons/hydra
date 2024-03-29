// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mcfs

import (
	"context"
	"syscall"
	"time"
	"unsafe"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/materials-commons/hydra/pkg/clog"

	"github.com/hanwen/go-fuse/v2/fuse"
)

// Allocate implements pre-allocating blocks for a file
func (f *BaseFileHandle) Allocate(ctx context.Context, off uint64, sz uint64, mode uint32) syscall.Errno {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	clog.Global().Debug("BaseFileHandle.Allocate")
	err := syscall.Fallocate(f.Fd, mode, int64(off), int64(sz))
	if err != nil {
		return fs.ToErrno(err)
	}
	return fs.OK
}

// Utimens - file handle based version of FileHandleBridgeSystem.Utimens()
func (f *BaseFileHandle) utimens(a *time.Time, m *time.Time) syscall.Errno {
	var ts [2]syscall.Timespec
	ts[0] = fuse.UtimeToTimespec(a)
	ts[1] = fuse.UtimeToTimespec(m)
	err := futimens(int(f.Fd), &ts)
	return fs.ToErrno(err)
}

func setBlocks(out *fuse.Attr) {
	if out.Blksize > 0 {
		return
	}

	out.Blksize = 4096
	pages := (out.Size + 4095) / 4096
	out.Blocks = pages * 8
}

// futimens - futimens(3) calls utimensat(2) with "pathname" set to null and
// "flags" set to zero
func futimens(fd int, times *[2]syscall.Timespec) (err error) {
	_, _, e1 := syscall.Syscall6(syscall.SYS_UTIMENSAT, uintptr(fd), 0, uintptr(unsafe.Pointer(times)), uintptr(0), 0, 0)
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return
}
