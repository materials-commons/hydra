package cmd

import (
	"fmt"
	"os"

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

	if err := setupMountPath(deps.mountPath); err != nil {
		return nil, err
	}

	createdFS, err := mcfs.CreateFS(mcfsDir, mcapi, newFileHandleFunc)
	if err != nil {
		return nil, fmt.Errorf("unable to create filesystem: %w", err)
	}

	rawfs := fs.NewNodeFS(createdFS, &fs.Options{})
	fuseServer, err := fuse.NewServer(rawfs, deps.mountPath, &fuse.MountOptions{Name: "mcfs"})
	if err != nil {
		return nil, fmt.Errorf("unable to create fuse server: %w", err)
	}

	return fuseServer, nil
}

func setupMountPath(mountPath string) error {
	_, err := os.Stat(mountPath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(mountPath, 0755)
		if err != nil {
			return fmt.Errorf("unable to create %s: %w", mountPath, err)
		}
	}

	return nil
}
