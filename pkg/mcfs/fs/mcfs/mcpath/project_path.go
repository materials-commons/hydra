package mcpath

import (
	"fmt"
	"path/filepath"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
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

	transferKey string

	stors *stor.Stors
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

func (p *ProjectPath) TransferKey() string {
	return p.transferKey
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

func (p *ProjectPath) TransferRequest() *mcmodel.TransferRequest {
	return nil
}

func (p *ProjectPath) PathType() PathType {
	return p.pathType
}

func (p *ProjectPath) Lookup() (*mcmodel.File, error) {
	switch p.pathType {
	case BadIDPath:
		return nil, fmt.Errorf("bad id path: %s", p.fullPath)

	case RootBasePath:
		// Return data on the root node
		return nil, fmt.Errorf("root not supported")

	case ProjectBasePath:
		// 	Return data on the project
		return p.lookupProject()

	case UserBasePath:
		// Return data on the user
		return p.lookupUser()

	default:
		f, err := p.stors.FileStor.GetFileByPath(p.projectID, p.projectPath)
		return f, err
	}
}

func (p *ProjectPath) lookupProject() (*mcmodel.File, error) {
	transferRequests, err := p.stors.TransferRequestStor.GetTransferRequestsForProject(p.projectID)
	switch {
	case err != nil:
		return nil, err

	case len(transferRequests) == 0:
		return nil, fmt.Errorf("no such path: %s", p.fullPath)

	default:
		// Found at least one transfer request for the project
		f := &mcmodel.File{
			Name:      fmt.Sprintf("%d", p.projectID),
			MimeType:  "directory",
			Path:      fmt.Sprintf("/%d", p.projectID),
			Directory: &mcmodel.File{Path: "/", Name: "/", MimeType: "directory"},
		}
		return f, nil
	}
}

func (p *ProjectPath) lookupUser() (*mcmodel.File, error) {
	// If we are here then the project has been verified, so we need to make sure that the
	// user exists
	tr, err := p.stors.TransferRequestStor.GetTransferRequestForProjectAndUser(p.projectID, p.userID)
	if err != nil {
		return nil, err
	}

	if tr == nil {
		return nil, fmt.Errorf("no such active user %d for project %d", p.userID, p.projectID)
	}

	f := &mcmodel.File{
		Name:     fmt.Sprintf("%d", p.userID),
		MimeType: "directory",
		Path:     fmt.Sprintf("/%d/%d", p.projectID, p.userID),
		Directory: &mcmodel.File{
			Path:     fmt.Sprintf("/%d", p.projectID),
			Name:     fmt.Sprintf("%d", p.projectID),
			MimeType: "directory",
		},
	}

	return f, nil
}

func (p *ProjectPath) List() ([]mcmodel.File, error) {
	switch p.pathType {
	case BadIDPath:
		return nil, fmt.Errorf("bad id path: %s", p.fullPath)
	case RootBasePath:
		// Return the list of projects that have transfer requests
		return p.listActiveProjects()
	case ProjectBasePath:
		// Return the list of users that have transfer requests for this project
		return p.listActiveUsersForProject()
	default:
		// Return directory contents for that /project/user/rest/of/project/path
		return p.listProjectDirectory()
	}
}

func (p *ProjectPath) listActiveProjects() ([]mcmodel.File, error) {
	transferRequests, err := p.stors.TransferRequestStor.ListTransferRequests()
	if err != nil {
		return nil, err
	}

	// first get a list of unique projects. There could be multiple transfer
	// requests for a single project, so build a hashmap of the projects. It
	// doesn't matter if there are multiple transfer requests per project,
	// since we just need the unique projects.
	uniqueProjects := make(map[int]mcmodel.TransferRequest)
	for _, tr := range transferRequests {
		uniqueProjects[tr.ProjectID] = tr
	}

	// Now build out the directories for the projects. We could
	// either use the key (the project id) or the transfer request
	// and get the project id from the transfer request. In this
	// code we used the transfer request, and then got the project
	// id from it.
	inDir := &mcmodel.File{Path: "/", MimeType: "directory"}
	var dirEntries []mcmodel.File
	for _, tr := range uniqueProjects {
		entry := mcmodel.File{
			Directory: inDir,
			Name:      fmt.Sprintf("%d", tr.ProjectID),
			Path:      fmt.Sprintf("/%d", tr.ProjectID),
			MimeType:  "directory",
		}
		dirEntries = append(dirEntries, entry)
	}
	return dirEntries, nil
}

func (p *ProjectPath) listActiveUsersForProject() ([]mcmodel.File, error) {
	transferRequests, err := p.stors.TransferRequestStor.ListTransferRequests()
	if err != nil {
		return nil, err
	}

	inDir := &mcmodel.File{Path: fmt.Sprintf("/%d", p.projectID), MimeType: "directory"}
	var dirEntries []mcmodel.File

	for _, tr := range transferRequests {
		if tr.ProjectID == p.projectID {
			entry := mcmodel.File{
				Directory: inDir,
				Name:      fmt.Sprintf("%d", tr.OwnerID),
				Path:      fmt.Sprintf("/%d/%d", tr.ProjectID, tr.OwnerID),
				MimeType:  "directory",
			}
			dirEntries = append(dirEntries, entry)
		}
	}

	if len(dirEntries) == 0 {
		return nil, fmt.Errorf("no such project: %d", p.projectID)
	}

	return dirEntries, nil
}

func (p *ProjectPath) listProjectDirectory() ([]mcmodel.File, error) {
	dir, err := p.stors.FileStor.GetDirByPath(p.projectID, p.projectPath)
	if err != nil {
		return nil, err
	}

	transferRequest, err := p.stors.TransferRequestStor.
		GetTransferRequestForProjectAndUser(p.projectID, p.userID)
	if err != nil {
		return nil, err
	}

	// Make list directory to a pointer for transferRequest?
	dirEntries, err := p.stors.TransferRequestStor.ListDirectory(dir, transferRequest)

	inDir := &mcmodel.File{Path: p.projectPath, MimeType: "directory"}
	for _, entry := range dirEntries {
		entry.Directory = inDir
	}

	return dirEntries, nil
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
	p := NewProjectPathParser(nil)
	projPath, err := p.Parse(path)
	if err != nil {
		return ""
	}
	return projPath.ProjectPath()
}

// Join takes a path that contains the project/user portions and returns
// the project path portion joined with elements. For example
// "/project-uuid/user-uuid/rest/of/path" joined with "dir1", "file.txt"
// will return "/rest/of/path/dir1/file.txt".
func Join(path string, elements ...string) string {
	p := NewProjectPathParser(nil)
	projPath, err := p.Parse(path)
	if err != nil {
		return ""
	}

	asProjPath := projPath.(*ProjectPath)
	return asProjPath.Join(elements...)
}
