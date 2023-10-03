package mcpath

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type ProjectPathParser struct {
	stors *stor.Stors
}

func NewProjectPathParser(stors *stor.Stors) *ProjectPathParser {
	return &ProjectPathParser{stors: stors}
}

// Parse takes a path containing the project and user uuid and creates
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
func (p *ProjectPathParser) Parse(path string) (Path, error) {
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
		return projPath, nil
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
		return p.parse2PartPath(pathParts, projPath)

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
		return p.parse3PartPath(pathParts, projPath)

	default:
		// If we are here then the user has a path into the project, either
		// like /123/456/, or /123/456/some/directory/path/or/file
		return p.parseGreaterThan3PartPath(pathParts, projPath)
	}
}

func (p *ProjectPathParser) parse2PartPath(pathParts []string, projPath *ProjectPath) (*ProjectPath, error) {
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
		return projPath, fmt.Errorf("projectid is not integer: %s", pathParts[1])
	}
	// The project id is good
	projPath.projectID = id
	projPath.pathType = ProjectBasePath
	return projPath, nil
}

func (p *ProjectPathParser) parse3PartPath(pathParts []string, projPath *ProjectPath) (*ProjectPath, error) {
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
			return projPath, fmt.Errorf("projectid is not integer: %s", pathParts[1])
		}
		// The project id is good
		projPath.projectID = id
		projPath.pathType = ProjectBasePath
		return projPath, nil
	}

	// If we are here then the path looks like /123/456
	if id, err = strconv.Atoi(pathParts[1]); err != nil {
		// Project id isn't numeric
		projPath.pathType = BadIDPath
		return projPath, fmt.Errorf("projectid is not integer: %s", pathParts[1])
	}

	projPath.projectID = id // Save the project id
	if id, err = strconv.Atoi(pathParts[2]); err != nil {
		// project id was numeric but user id isn't so this
		// is a bad path
		projPath.pathType = BadIDPath
		return projPath, fmt.Errorf("userid is not integer: %s", pathParts[2])
	}

	// If we are here then both the proj id and the user id were
	// numeric. So a bit more work to fill out projPath.
	projPath.userID = id
	projPath.pathType = UserBasePath

	// Transfer is same as full path, while project path is "/"
	projPath.transferBase = projPath.fullPath
	projPath.projectPath = "/"
	return projPath, nil
}

func (p *ProjectPathParser) parseGreaterThan3PartPath(pathParts []string, projPath *ProjectPath) (*ProjectPath, error) {
	// This is a full path that includes a project and a user id. The path looks something
	// like /123/456/, or /123/456/some/directory/path/or/file
	var (
		err               error
		projectID, userID int
	)

	if projectID, err = strconv.Atoi(pathParts[1]); err != nil {
		projPath.pathType = BadIDPath
		return projPath, fmt.Errorf("project id is not integer: %s", pathParts[1])
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
	return projPath, nil
}
