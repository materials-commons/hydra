package cmd

import (
	"fmt"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/fsstate"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/mcpath"
)

type FSDependencies struct {
	stors     *stor.Stors
	fsState   *fsstate.FSState
	mcfsDir   string
	mountPath string
}

func createFS(deps FSDependencies) (*fuse.Server, error) {
	pathParser := mcpath.NewTransferPathParser(deps.stors, deps.fsState.TransferRequestCache)
	mcapi := mcfs.NewLocalMCFSApi(deps.stors, deps.fsState.TransferStateTracker, pathParser, deps.mcfsDir)
	handleFactory := mcfs.NewMCFileHandlerFactory(mcapi, deps.fsState.TransferStateTracker, pathParser, deps.fsState.ActivityTracker)
	newFileHandleFunc := func(fd, flags int, path string, file *mcmodel.File) fs.FileHandle {
		return handleFactory.NewFileHandle(fd, flags, path, file)
	}

	createdFS, err := mcfs.CreateFS(mcfsDir, mcapi, newFileHandleFunc)
	if err != nil {
		return nil, fmt.Errorf("unable to create filesystem: %s", err)
	}

	rawfs := fs.NewNodeFS(createdFS, &fs.Options{})
	fuseServer, err := fuse.NewServer(rawfs, deps.mountPath, &fuse.MountOptions{Name: "mcfs"})
	if err != nil {
		return nil, fmt.Errorf("unable to create fuse server: %s", err)
	}

	return fuseServer, nil
}
