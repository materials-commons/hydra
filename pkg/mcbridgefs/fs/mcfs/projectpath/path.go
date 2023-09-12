package projectpath

import (
	"path/filepath"
	"strconv"
	"strings"
)

// ProjectPath represents the different parts of a project path in the file system.
// Each project/user gets a unique path for upload/download. The path starts with
// the project ID and user ID, eg /25/301. The rest of the path is the directory
// tree for that project. In the database paths are stored without the project/user
// id, eg /25/301/dir1/file.txt has path /dir1/file.txt.
//
// The methods for ProjectPath help with these two representations
type ProjectPath struct {
	// The id for the project; derived from the path
	ProjectID int

	// id for the user; derived from the path
	UserID int

	// The project path, ie after remove the project-id and user-id portions
	ProjectPath string

	// The TransferBase is the project user path. For example if the path
	// is /25/301/rest/of/path, then TransferBase is /25/301
	TransferBase string

	// The full path, containing the project-id and the user-id
	FullPath string
}

// NewProjectPath takes a path containing the project and user uuid and creates
// a ProjectPath structure containing the various parts of the path. A path
// consists of /project-id/user-id/rest/of/path. From this path it constructs
// ProjectPath that would look as follows for /25/301/rest/of/path
//
//	&ProjecPath{
//	    ProjectID: 25
//	    UserUUID: 301
//	    ProjectPath: /rest/of/path
//	    FullPath: /25/301/rest/of/path
//	}
func NewProjectPath(path string) *ProjectPath {
	pathParts := strings.Split(path, "/")
	// pathParts[0] = ""
	// pathParts[1] = project-id
	// pathParts[2] = user-id
	// pathParts[3:] = path to use for project path

	if len(pathParts) < 3 {
		return &ProjectPath{
			ProjectID:    -1,
			UserID:       -1,
			ProjectPath:  "/",
			TransferBase: "/",
			FullPath:     filepath.Clean(path),
		}
	}

	// Default project path to "/" for the case where other file or directory path is
	// included past the userid.
	projectPath := "/"

	if len(pathParts) > 3 {
		// The project root starts with a slash, so add a "/" into the list of
		// path parts we are going to join.
		pathPieces := append([]string{"/"}, pathParts[3:]...)
		projectPath = filepath.Join(pathPieces...)
	}

	transferBase := filepath.Join("/", pathParts[1], pathParts[2])

	var (
		projectID, userID int
		err               error
	)

	if projectID, err = strconv.Atoi(pathParts[1]); err != nil {
		projectID = -1
	}

	if userID, err = strconv.Atoi(pathParts[2]); err != nil {
		userID = -1
	}

	return &ProjectPath{
		ProjectID:    projectID,
		UserID:       userID,
		ProjectPath:  projectPath,
		TransferBase: transferBase,
		FullPath:     filepath.Clean(path),
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
// example if ProjectPath.FullPath is "/25/301/dir1/dir2", and you join "dir3", "file.txt"
// it will return "/25/301/dir1/dir2/dir3/file.txt".
func (p *ProjectPath) FullPathJoin(elements ...string) string {
	pathPieces := append([]string{p.FullPath}, elements...)
	return filepath.Join(pathPieces...)
}

// ToProjectPath takes a path that contains the project/user portions and returns
// the project path. For example "/25/301/rest/of/path" will return
// "/rest/of/path".
func ToProjectPath(path string) string {
	p := NewProjectPath(path)
	return p.ProjectPath
}

// TransferBase takes a path that contains the project/user portions and returns
// the TransferBase. For example "/25/301/rest/of/path" will return
// "/25/301".
func TransferBase(path string) string {
	p := NewProjectPath(path)
	return p.TransferBase
}

// ProjectID takes a path that contains the project/user portions and returns
// the project-id. For example "/25/301/rest/of/path" will return 25.
func ProjectID(path string) (id int) {
	p := NewProjectPath(path)
	return p.ProjectID
}

// UserID takes a path that contains the project/user portions and returns
// the user-id. For example "/25/301/rest/of/path" will return 301.
func UserID(path string) (id int) {
	p := NewProjectPath(path)
	return p.UserID
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
