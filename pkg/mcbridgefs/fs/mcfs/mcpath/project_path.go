package mcpath

import (
	"path/filepath"
)

//type pathTypeEnum int

const (
	RootBasePath     PathType = 10
	ProjectBasePath  PathType = 11
	UserBasePath     PathType = 12
	CompleteBasePath PathType = 13
	BadIDPath        PathType = 14
)

// ProjectPath represents the different parts of a project path in the file system.
// Each project/user gets a unique path for upload/download. The path starts with
// the project ID and user ID, eg /25/301. The rest of the path is the directory
// tree for that project. In the database paths are stored without the project/user
// id, eg /25/301/dir1/file.txt has path /dir1/file.txt.
//
// The methods for ProjectPath help with these two representations
type ProjectPath struct {
	// The type of path this represents
	pathType PathType

	// The id for the project; derived from the path
	projectID int

	// id for the user; derived from the path
	userID int

	// The project path, ie after remove the project-id and user-id portions
	projectPath string

	// The TransferBase is the project user path. For example if the path
	// is /25/301/rest/of/path, then TransferBase is /25/301
	transferBase string

	// The full path, containing the project-id and the user-id
	fullPath string
}

func (p *ProjectPath) ProjectID() int {
	return p.projectID
}
func (p *ProjectPath) UserID() int {
	return p.userID
}
func (p *ProjectPath) TransferID() int {
	return -1
}
func (p *ProjectPath) TransferUUID() string {
	return ""
}
func (p *ProjectPath) ProjectPath() string {
	return p.projectPath
}
func (p *ProjectPath) FullPath() string {
	return p.fullPath
}
func (p *ProjectPath) TransferBase() string {
	return p.transferBase
}
func (p *ProjectPath) PathType() PathType {
	return p.pathType
}

// Join will return the joined path elements onto the ProjectPath.ProjectPath, for
// example if ProjectPath.ProjectPath is "/dir1/dir2", and you join "dir3", "file.txt"
// it will return "/dir1/dir2/dir3/file.txt".
func (p *ProjectPath) Join(elements ...string) string {
	pathPieces := append([]string{p.projectPath}, elements...)
	return filepath.Join(pathPieces...)
}

// FullPathJoin will return the joined path elements onto the ProjectPath.FullPath, for
// example if ProjectPath.FullPath is "/25/301/dir1/dir2", and you join "dir3", "file.txt"
// it will return "/25/301/dir1/dir2/dir3/file.txt".
func (p *ProjectPath) FullPathJoin(elements ...string) string {
	pathPieces := append([]string{p.fullPath}, elements...)
	return filepath.Join(pathPieces...)
}

// ToProjectPath takes a path that contains the project/user portions and returns
// the project path. For example "/25/301/rest/of/path" will return
// "/rest/of/path".
func ToProjectPath(path string) string {
	p := NewProjectPathParser()
	projPath, err := p.Parse(path)
	if err != nil {
		return ""
	}
	return projPath.ProjectPath()
}

// TransferBase takes a path that contains the project/user portions and returns
// the TransferBase. For example "/25/301/rest/of/path" will return
// "/25/301".
func TransferBase(path string) string {
	p := NewProjectPathParser()
	projPath, err := p.Parse(path)
	if err != nil {
		return ""
	}
	return projPath.TransferBase()
}

// ProjectID takes a path that contains the project/user portions and returns
// the project-id. For example "/25/301/rest/of/path" will return 25.
func ProjectID(path string) (id int) {
	p := NewProjectPathParser()
	projPath, err := p.Parse(path)
	if err != nil {
		return -1
	}
	return projPath.ProjectID()
}

// UserID takes a path that contains the project/user portions and returns
// the user-id. For example "/25/301/rest/of/path" will return 301.
func UserID(path string) (id int) {
	p := NewProjectPathParser()
	projPath, err := p.Parse(path)
	if err != nil {
		return -1
	}
	return projPath.UserID()
}

// Join takes a path that contains the project/user portions and returns
// the project path portion joined with elements. For example
// "/project-uuid/user-uuid/rest/of/path" joined with "dir1", "file.txt"
// will return "/rest/of/path/dir1/file.txt".
func Join(path string, elements ...string) string {
	p := NewProjectPathParser()
	projPath, err := p.Parse(path)
	if err != nil {
		return ""
	}

	asProjPath := projPath.(*ProjectPath)
	return asProjPath.Join(elements...)
}

// FullPathJoin takes a path that contains the project/user portions and returns
// the project path portion joined with elements. For example
// "/project-uuid/user-uuid/rest/of/path" joined with "dir1", "file.txt"
// will return "/project-uuid/user-uuid/rest/of/path/dir1/file.txt".
func FullPathJoin(path string, elements ...string) string {
	p := NewProjectPathParser()
	projPath, err := p.Parse(path)
	if err != nil {
		return ""
	}
	asProjPath := projPath.(*ProjectPath)
	return asProjPath.FullPathJoin(elements...)
}
