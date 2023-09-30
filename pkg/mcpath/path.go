package mcpath

type Parser interface {
	Parse(path string) (Path, error)
}

type Path interface {
	ProjectID() int
	UserID() int
	TransferID() int
	TransferUUID() string
	ProjectPath() string
	FullPath() string
	TransferBase() string
}
