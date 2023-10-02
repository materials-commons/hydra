package mcpath

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
	TransferUUID() string
	ProjectPath() string
	FullPath() string
	TransferBase() string
	PathType() PathType
}

type PathType int

const (
	RootPath    PathType = 1
	ContextPath PathType = 2
	ProjectPath PathType = 3
	BadPath     PathType = 4
)
