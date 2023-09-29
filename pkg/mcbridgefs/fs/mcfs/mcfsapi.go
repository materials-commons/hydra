package mcfs

import (
	"syscall"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

// MCFSApi is the interface into Materials Commons that supports the file system. It follows
// a naming convention that mostly matches the FUSE call. So for example Create, Open, Release,
// Lookup, Readdir, Mkdir, FTruncate correspond to the FUSE calls.
type MCFSApi interface {
	// Create creates a new file database entry.
	Create(path string) (*mcmodel.File, error)

	// Open will create a new mcmodel.File or return an existing one depending on
	// the flags passed in.
	Open(path string, flags int) (f *mcmodel.File, isNewFile bool, err error)

	// Release releases (closes) a file and updates metadata about the file in MC.
	Release(path string, size uint64) error

	// Lookup returns a file entry if it exists.
	Lookup(path string) (*mcmodel.File, error)

	// Readdir returns a list of mcmodel.File entries for a directory.
	Readdir(path string) ([]mcmodel.File, error)

	// Mkdir creates a new directory in MC.
	Mkdir(path string) (*mcmodel.File, error)

	// GetRealPath will take a MCFS path and return the path to the real underlying file (the UUID based path).
	GetRealPath(path string) (realpath string, err error)

	// GetKnownFileRealPath will return the real underlying file path (the UUID based path) only for files
	// that are in the KnownFile tracker.
	GetKnownFileRealPath(path string) (string, error)

	// FTruncate will truncate the real underlying file (the UUID based one) and return
	// Stat info on it.
	FTruncate(path string, size uint64) (error, *syscall.Stat_t)
}
