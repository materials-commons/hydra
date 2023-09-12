package mcfs

import (
	"fmt"
	"mime"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/materials-commons/hydra/pkg/mcbridgefs/fs/mcfs/projectpath"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

// MCApi is the file system interface into Materials Commons. It has little knowledge of
// FUSE. It understands the Materials Commons calls to make to achieve certain file system
// operations, and returns the results in a way that the node can pass back.
type MCApi struct {
	stors             *stor.Stors
	knownFilesTracker *KnownFilesTracker
}

func NewMCApi(stors *stor.Stors, tracker *KnownFilesTracker) *MCApi {
	return &MCApi{
		stors:             stors,
		knownFilesTracker: tracker,
	}
}

func (fs *MCApi) Readdir(path string) ([]mcmodel.File, error) {
	projPath := projectpath.NewProjectPath(path)

	dir, err := fs.stors.FileStor.GetDirByPath(projPath.ProjectID, projPath.ProjectPath)
	if err != nil {
		return nil, err
	}

	transferRequest, err := fs.stors.TransferRequestStor.GetTransferRequestByProjectAndUser(projPath.ProjectID, projPath.UserID)
	if err != nil {
		return nil, err
	}

	// Make list directory to a pointer for transferRequest?
	dirEntries, err := fs.stors.TransferRequestStor.ListDirectory(dir, transferRequest)

	inDir := &mcmodel.File{Path: projPath.ProjectPath}
	for _, entry := range dirEntries {
		entry.Directory = inDir
	}

	return dirEntries, nil
}

func (fs *MCApi) GetRealPath(path string, mcfsRoot string) (realpath string, err error) {
	if file := fs.knownFilesTracker.GetFile(path); file != nil {
		// Found known file, so return it's real path
		return file.ToUnderlyingDirPath(mcfsRoot), nil
	}

	// Didn't find a previously opened file, so look up file.
	projPath := projectpath.NewProjectPath(path)
	file, err := fs.stors.FileStor.GetFileByPath(projPath.ProjectID, projPath.ProjectPath)
	if err != nil {
		return "", err
	}

	return file.ToUnderlyingFilePath(mcfsRoot), nil
}

func (fs *MCApi) Lookup(path string) (*mcmodel.File, error) {
	pathType, err := fs.pathType(path)
	if err != nil {
		return nil, err
	}
	switch pathType {
	case RootBasePath:
		// Return data on the root node
		return nil, fmt.Errorf("root not supported")
	case ProjectBasePath:
		// 	Return data on the project
	case UserBasePath:
		// Return data on the user
	default:
		projPath := projectpath.NewProjectPath(path)
		f, err := fs.stors.FileStor.GetFileByPath(projPath.ProjectID, projPath.ProjectPath)
		return f, err
	}

	projPath := projectpath.NewProjectPath(path)
	f, err := fs.stors.FileStor.GetFileByPath(projPath.ProjectID, projPath.ProjectPath)

	return f, err
}

type pathTypeEnum int

const (
	RootBasePath     pathTypeEnum = 1
	ProjectBasePath  pathTypeEnum = 2
	UserBasePath     pathTypeEnum = 3
	CompleteBasePath pathTypeEnum = 4
	BadIDPath        pathTypeEnum = 5
)

func (fs *MCApi) pathType(path string) (pathTypeEnum, error) {
	if path == "/" {
		return RootBasePath, nil
	}
	pathParts := strings.Split(path, "/")
	switch len(pathParts) {
	case 2:
		// Path looks something like /123, when split we have
		// pathParts[0] = ""
		// pathParts[1] = "123".
		// This is a project based lookup
		if _, err := strconv.Atoi(pathParts[1]); err != nil {
			return BadIDPath, err
		}
		return ProjectBasePath, nil
	case 3:
		// There are two cases for the path, it could look like
		// /123/ or /123/456. The first instance is a project
		// based lookup, the second is a user based look up.
		if pathParts[2] == "" {
			if _, err := strconv.Atoi(pathParts[1]); err != nil {
				return BadIDPath, err
			}
			return ProjectBasePath, nil
		}
		if _, err := strconv.Atoi(pathParts[2]); err != nil {
			return BadIDPath, err
		}

		return UserBasePath, nil
	default:
		// Other cases can be parsed out by projectpath
		return CompleteBasePath, nil
	}
}

func (fs *MCApi) Mkdir(path string) (*mcmodel.File, error) {
	projPath := projectpath.NewProjectPath(path)
	parentDir, err := fs.stors.FileStor.GetFileByPath(projPath.ProjectID, filepath.Dir(projPath.ProjectPath))
	if err != nil {
		return nil, err
	}

	return fs.stors.FileStor.CreateDirectory(parentDir.ID, projPath.ProjectID, projPath.UserID, projPath.ProjectPath, filepath.Base(projPath.ProjectPath))
}

func (fs *MCApi) Create(path string) (*mcmodel.File, error) {
	projPath := projectpath.NewProjectPath(path)
	if file := fs.knownFilesTracker.GetFile(projPath.FullPath); file != nil {
		// This should not happen - Create was called on a file that the file
		// system is already tracking as opened.
		return nil, fmt.Errorf("file found on create: %s", path)
	}

	f, err := fs.createNewFile(projPath)
	fs.knownFilesTracker.Store(path, f)

	return f, err
}

func (fs *MCApi) Open(path string, isReadOnly bool) (f *mcmodel.File, isNewFile bool, err error) {
	projPath := projectpath.NewProjectPath(path)
	f = fs.knownFilesTracker.GetFile(path)
	if f != nil {
		// Existing file found
		return f, false, nil
	}

	if isReadOnly {
		// If we are here then this is a request to open a file for read. The file
		// needs to exist.
		f, err = fs.stors.FileStor.GetFileByPath(projPath.ProjectID, projPath.ProjectPath)
		return f, false, err
	}

	// If we are here then the file wasn't found in the list of already opened
	// files, so we need to create the file.
	f, err = fs.createNewFileVersion(projPath)
	return f, true, err
}

// createNewFile will create a new mcmodel.File entry for the directory associated
// with the Node. It will create the directory where the file can be written to.
func (fs *MCApi) createNewFile(projPath *projectpath.ProjectPath) (*mcmodel.File, error) {
	dir, err := fs.stors.FileStor.GetDirByPath(projPath.ProjectID, filepath.Dir(projPath.ProjectPath))
	if err != nil {
		return nil, err
	}

	tr, err := fs.stors.TransferRequestStor.GetTransferRequestByProjectAndUser(projPath.ProjectID, projPath.UserID)
	if err != nil {
		return nil, err
	}

	name := filepath.Base(projPath.ProjectPath)

	file := &mcmodel.File{
		ProjectID:   projPath.ProjectID,
		Name:        name,
		DirectoryID: dir.ID,
		Size:        0,
		Checksum:    "",
		MimeType:    determineMimeType(name),
		OwnerID:     projPath.UserID,
		Current:     false,
	}

	return fs.stors.TransferRequestStor.CreateNewFile(file, dir, tr)
}

// createNewFileVersion creates a new file version if there isn't already a version of the file
// associated with this transfer request instance. It checks the knownFilesTracker to determine
// if a new version has already been created. If a new version was already created then it will return
// that version. Otherwise, it will create a new version and add it to the OpenedFilesTracker. In
// addition, when a new version is created, the associated on disk directory is created.
func (fs *MCApi) createNewFileVersion(projPath *projectpath.ProjectPath) (*mcmodel.File, error) {
	var err error

	name := filepath.Base(projPath.ProjectPath)

	dir, err := fs.stors.FileStor.GetDirByPath(projPath.ProjectID, filepath.Dir(projPath.ProjectPath))
	if err != nil {
		return nil, err
	}

	tr, err := fs.stors.TransferRequestStor.GetTransferRequestByProjectAndUser(projPath.ProjectID, projPath.UserID)
	if err != nil {
		return nil, err
	}

	// There isn't an existing upload, so create a new one
	f := &mcmodel.File{
		ProjectID:   projPath.ProjectID,
		Name:        name,
		DirectoryID: dir.ID,
		Size:        0,
		Checksum:    "",
		MimeType:    determineMimeType(name),
		OwnerID:     projPath.UserID,
		Current:     false,
	}

	f, err = fs.stors.TransferRequestStor.CreateNewFile(f, dir, tr)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// determineMimeType ... Move this into a utility package.
func determineMimeType(name string) string {
	mimeType := mime.TypeByExtension(filepath.Ext(name))
	if mimeType == "" {
		return "unknown"
	}

	mediatype, _, err := mime.ParseMediaType(mimeType)
	if err != nil {
		// ParseMediaType returned an error, but TypeByExtension
		// returned a mime string. As a fallback let's remove
		// any parameters on the string (if there is a semicolon
		// it will be after the semicolon), and return everything
		// before the (optional) semicolon.
		semicolon := strings.Index(mimeType, ";")
		if semicolon == -1 {
			return strings.TrimSpace(mimeType)
		}

		return strings.TrimSpace(mimeType[:semicolon])
	}

	return strings.TrimSpace(mediatype)
}
