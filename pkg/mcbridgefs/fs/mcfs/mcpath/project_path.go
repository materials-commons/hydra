package mcpath

import (
	"path/filepath"
	"strconv"
	"strings"
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

// ParseProjectPath takes a path containing the project and user uuid and creates
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
func ParseProjectPath(path string) *ProjectPath {
	// Create initial ProjectPath and fill out with
	// default values. This will be further filled in
	// as we parse out the path components.
	projPath := &ProjectPath{
		pathType:     RootBasePath,
		projectID:    -1,
		userID:       -1,
		projectPath:  "/",
		transferBase: "/",
		fullPath:     filepath.Clean(path),
	}

	if path == "/" {
		return projPath
	}

	pathParts := strings.Split(projPath.fullPath, "/")

	// A fully formed path looks as follows:
	//   pathParts[0] = ""
	//   pathParts[1] = project-id
	//   pathParts[2] = user-id
	//   pathParts[3:] = path to use for project path
	//
	// However this may only be a partial path, so lets figure out
	// what we have, and fill in the appropriate fields in projPath.

	switch len(pathParts) {
	case 2:
		// This is a path containing only a project id.
		// The path looks something like /123, when split we have
		return parse2PartPath(pathParts, projPath)

	case 3:
		// This is a path containing both a project id and possibly a user id.
		//
		// There are two cases for the path, it could look like
		// /123/ or /123/456. The first instance is a project
		// based lookup, the second is a user based look up.
		//
		// pathParts[0] = ""
		// pathParts[1] = "123"
		// pathParts[2] = "" (path is /123/)
		//    or
		// pathParts[2] = "456" (path is /123/456)
		return parse3PartPath(pathParts, projPath)

	default:
		// If we are here then the user has a path into the project, either
		// like /123/456/, or /123/456/some/directory/path/or/file
		return parseGreaterThan3PartPath(pathParts, projPath)
	}
}

func parse2PartPath(pathParts []string, projPath *ProjectPath) *ProjectPath {
	var (
		id  int
		err error
	)
	// This is a path containing only a project id.
	//
	// Path looks something like /123, when split we have
	// pathParts[0] = ""
	// pathParts[1] = "123".
	// This is a project based lookup
	if id, err = strconv.Atoi(pathParts[1]); err != nil {
		// This should have been a path to a project ID, but
		// the project id isn't an integer.
		projPath.pathType = BadIDPath
		return projPath
	}
	// The project id is good
	projPath.projectID = id
	projPath.pathType = ProjectBasePath
	return projPath
}

func parse3PartPath(pathParts []string, projPath *ProjectPath) *ProjectPath {
	// This is a path containing both a project id and possibly a user id.
	//
	// There are two cases for the path, it could look like
	// /123/ or /123/456. The first instance is a project
	// based lookup, the second is a user based look up.
	//
	// pathParts[0] = ""
	// pathParts[1] = "123"
	// pathParts[2] = "" (path is /123/)
	//    or
	// pathParts[2] = "456" (path is /123/456)
	var (
		id  int
		err error
	)

	if pathParts[2] == "" {
		// We are in the case where the path looks something like
		// /123/
		if id, err = strconv.Atoi(pathParts[1]); err != nil {
			// The project id isn't numeric
			projPath.pathType = BadIDPath
			return projPath
		}
		// The project id is good
		projPath.projectID = id
		projPath.pathType = ProjectBasePath
		return projPath
	}

	// If we are here then the path looks like /123/456
	if id, err = strconv.Atoi(pathParts[1]); err != nil {
		// Project id isn't numeric
		projPath.pathType = BadIDPath
		return projPath
	}

	projPath.projectID = id // Save the project id
	if id, err = strconv.Atoi(pathParts[2]); err != nil {
		// project id was numeric but user id isn't so this
		// is a bad path
		projPath.pathType = BadIDPath
		return projPath
	}

	// If we are here then both the proj id and the user id were
	// numeric. So a bit more work to fill out projPath.
	projPath.userID = id
	projPath.pathType = UserBasePath

	// Transfer is same as full path, while project path is "/"
	projPath.transferBase = projPath.fullPath
	projPath.projectPath = "/"
	return projPath
}

func parseGreaterThan3PartPath(pathParts []string, projPath *ProjectPath) *ProjectPath {
	// This is a full path that includes a project and a user id. The path looks something
	// like /123/456/, or /123/456/some/directory/path/or/file
	var (
		err               error
		projectID, userID int
	)

	if projectID, err = strconv.Atoi(pathParts[1]); err != nil {
		projPath.pathType = BadIDPath
		return projPath
	}

	projPath.projectID = projectID

	if userID, err = strconv.Atoi(pathParts[2]); err != nil {
		projPath.pathType = BadIDPath

		userID = -1
	}

	projPath.userID = userID
	projPath.pathType = CompleteBasePath
	pathPieces := append([]string{"/"}, pathParts[3:]...)
	projPath.projectPath = filepath.Join(pathPieces...)
	projPath.transferBase = filepath.Join("/", pathParts[1], pathParts[2])
	return projPath
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
	p := ParseProjectPath(path)
	return p.projectPath
}

// TransferBase takes a path that contains the project/user portions and returns
// the TransferBase. For example "/25/301/rest/of/path" will return
// "/25/301".
func TransferBase(path string) string {
	p := ParseProjectPath(path)
	return p.transferBase
}

// ProjectID takes a path that contains the project/user portions and returns
// the project-id. For example "/25/301/rest/of/path" will return 25.
func ProjectID(path string) (id int) {
	p := ParseProjectPath(path)
	return p.projectID
}

// UserID takes a path that contains the project/user portions and returns
// the user-id. For example "/25/301/rest/of/path" will return 301.
func UserID(path string) (id int) {
	p := ParseProjectPath(path)
	return p.userID
}

// Join takes a path that contains the project/user portions and returns
// the project path portion joined with elements. For example
// "/project-uuid/user-uuid/rest/of/path" joined with "dir1", "file.txt"
// will return "/rest/of/path/dir1/file.txt".
func Join(path string, elements ...string) string {
	p := ParseProjectPath(path)
	return p.Join(elements...)
}

// FullPathJoin takes a path that contains the project/user portions and returns
// the project path portion joined with elements. For example
// "/project-uuid/user-uuid/rest/of/path" joined with "dir1", "file.txt"
// will return "/project-uuid/user-uuid/rest/of/path/dir1/file.txt".
func FullPathJoin(path string, elements ...string) string {
	p := ParseProjectPath(path)
	return p.FullPathJoin(elements...)
}
