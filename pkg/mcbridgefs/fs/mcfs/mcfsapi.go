package mcfs

import (
	"syscall"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type MCFSApi interface {
	Create(path string) (*mcmodel.File, error)
	Open(path string, flags int) (f *mcmodel.File, isNewFile bool, err error)
	Release(path string, size uint64) error
	Lookup(path string) (*mcmodel.File, error)
	Readdir(path string) ([]mcmodel.File, error)
	Mkdir(path string) (*mcmodel.File, error)
	GetRealPath(path string, mcfsRoot string) (realpath string, err error)
	GetKnownFileRealPath(path, mcfsRoot string) (string, error)
	FTruncate(path, mcfsRoot string, size uint64) (error, *syscall.Stat_t)
}
