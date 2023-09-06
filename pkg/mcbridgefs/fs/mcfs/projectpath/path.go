package projectpath

import (
	"path/filepath"
	"strings"
)

// ProjectPath represents the different parts of a project path in the file system.
// Each project/user gets a unique path for upload/download. The path starts with
// the project UUID and user UUID, eg /project-uuid/user-uuid. The rest of the
// path is the directory tree for that project. In the database paths are stored
// without the project/user uuid, eg /project-uuid/user-uuid/dir1/file.txt has
// path /dir1/file.txt.
//
// The methods for ProjectPath help with these two representations
type ProjectPath struct {
	// The uuid for the project; derived from the path
	ProjectUUID string

	// uuid for the user; derived from the path
	UserUUID string

	// The project path, ie after remove the project-uuid and user-uuid portions
	ProjectPath string

	// The full path, containing the project-uuid and the user-uuid
	FullPath string
}

// NewProjectPath takes a path containing the project and user uuid and creates
// a ProjectPath structure containing the various parts of the path. A path
// consists of /project-uuid/user-uuid/rest/of/path. From this path it constructs
// ProjectPath that would looks as follows:
//
//	&ProjecPath{
//	    ProjectUUID: project-uuid
//	    UserUUID: user-uuid
//	    ProjectPath: /rest/of/path
//	    FullPath: /project-uuid/user-uuid/rest/of/path
//	}
func NewProjectPath(path string) *ProjectPath {
	pathParts := strings.Split(path, "/")
	// pathParts[0] = ""
	// pathParts[1] = project-uuid
	// pathParts[2] = user-uuid
	// pathParts[...] = path to use for project path

	// The project root starts with a slash, so add a "/" into the list of
	// path parts we are going to join.
	pathPieces := append([]string{"/"}, pathParts[3:]...)
	projectPath := filepath.Join(pathPieces...)

	return &ProjectPath{
		ProjectUUID: pathParts[1],
		UserUUID:    pathParts[2],
		ProjectPath: projectPath,
		FullPath:    filepath.Clean(path),
	}
}

// Join will return the joined path elements onto the ProjectPath.ProjectPath, for
// example if ProjectPath.ProjectPath is "/dir1/dir2", and you join "dir3", "file.txt"
// it will return "/dir1/dir2/dir3/file.txt".
func (p *ProjectPath) Join(elements ...string) string {
	pathPieces := append([]string{p.ProjectPath}, elements...)
	return filepath.Join(pathPieces...)
}

// FullPathJoin will return the joined path elements onto the ProjectPath.FullPath, for
// example if ProjectPath.FullPath is "/proj-uuid/user-uuid/dir1/dir2", and you join
// "dir3", "file.txt" it will return "/proj-uuid/user-uuid/dir1/dir2/dir3/file.txt".
func (p *ProjectPath) FullPathJoin(elements ...string) string {
	pathPieces := append([]string{p.FullPath}, elements...)
	return filepath.Join(pathPieces...)
}

// ToProjectPath takes a path that contains the project/user portions and returns
// the project path. For example "/project-uuid/user-uuid/rest/of/path" will return
// "/rest/of/path".
func ToProjectPath(path string) string {
	p := NewProjectPath(path)
	return p.ProjectPath
}

// ProjectUUID takes a path that contains the project/user portions and returns
// the project-uuid. For example "/project-uuid/user-uuid/rest/of/path" will return
// project-uuid.
func ProjectUUID(path string) (uuid string) {
	p := NewProjectPath(path)
	return p.ProjectUUID
}

// UserUUID takes a path that contains the project/user portions and returns
// the user-uuid. For example "/project-uuid/user-uuid/rest/of/path" will return
// user-uuid.
func UserUUID(path string) (uuid string) {
	p := NewProjectPath(path)
	return p.UserUUID
}

// Join takes a path that contains the project/user portions and returns
// the project path portion joined with elements. For example
// "/project-uuid/user-uuid/rest/of/path" joined with "dir1", "file.txt"
// will return "/rest/of/path/dir1/file.txt".
func Join(path string, elements ...string) string {
	p := NewProjectPath(path)
	return p.Join(elements...)
}

// FullPathJoin takes a path that contains the project/user portions and returns
// the project path portion joined with elements. For example
// "/project-uuid/user-uuid/rest/of/path" joined with "dir1", "file.txt"
// will return "/project-uuid/user-uuid/rest/of/path/dir1/file.txt".
func FullPathJoin(path string, elements ...string) string {
	p := NewProjectPath(path)
	return p.FullPathJoin(elements...)
}
