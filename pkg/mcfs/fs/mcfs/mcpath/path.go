package mcpath

import (
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type Parser interface {
	Parse(path string) (Path, error)
}

type Releaser interface {
	Release(path string)
}

type ParserReleaser interface {
	Parser
	Releaser
}

type Path interface {
	ProjectID() int
	UserID() int
	TransferID() int
	TransferKey() string
	ProjectPath() string
	FullPath() string
	TransferBase() string
	PathType() PathType
	Lookup() (*mcmodel.File, error)
	List() ([]mcmodel.File, error)
}

type PathType int

const (
	RootPathType    PathType = 1
	ContextPathType PathType = 2
	ProjectPathType PathType = 3
	BadPathType     PathType = 4
)
