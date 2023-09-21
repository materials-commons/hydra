package mcfs

import (
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	"strings"
	"syscall"

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
	fmt.Printf("MCApi.Readdir: %q\n", path)

	projPath := projectpath.NewProjectPath(path)

	switch projPath.PathType {
	case projectpath.BadIDPath:
		return nil, fmt.Errorf("bad id path: %s", path)
	case projectpath.RootBasePath:
		// Return the list of projects that have transfer requests
		fmt.Println("  Readdir RootBasePath")
		return fs.listActiveProjects()
	case projectpath.ProjectBasePath:
		// Return the list of users that have transfer requests for this project
		fmt.Println("  Readdir ProjectBasePath")
		return fs.listActiveUsersForProject(path)
	default:
		// Return directory contents for that /project/user/rest/of/project/path
		fmt.Println("  Readdir default")
		return fs.listProjectDirectory(path)
	}
}

func (fs *MCApi) listActiveProjects() ([]mcmodel.File, error) {
	transferRequests, err := fs.stors.TransferRequestStor.ListTransferRequests()
	if err != nil {
		return nil, err
	}

	inDir := &mcmodel.File{Path: "/", MimeType: "directory"}
	var dirEntries []mcmodel.File
	for _, tr := range transferRequests {
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

func (fs *MCApi) listActiveUsersForProject(path string) ([]mcmodel.File, error) {
	projPath := projectpath.NewProjectPath(path)
	transferRequests, err := fs.stors.TransferRequestStor.ListTransferRequests()
	if err != nil {
		return nil, err
	}

	inDir := &mcmodel.File{Path: fmt.Sprintf("/%d", projPath.ProjectID), MimeType: "directory"}
	var dirEntries []mcmodel.File

	for _, tr := range transferRequests {
		if tr.ProjectID == projPath.ProjectID {
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
		return nil, fmt.Errorf("no such project: %d", projPath.ProjectID)
	}

	return dirEntries, nil
}

func (fs *MCApi) listProjectDirectory(path string) ([]mcmodel.File, error) {
	projPath := projectpath.NewProjectPath(path)

	dir, err := fs.stors.FileStor.GetDirByPath(projPath.ProjectID, projPath.ProjectPath)
	if err != nil {
		return nil, err
	}

	transferRequest, err := fs.stors.TransferRequestStor.GetTransferRequestForProjectAndUser(projPath.ProjectID, projPath.UserID)
	if err != nil {
		return nil, err
	}

	// Make list directory to a pointer for transferRequest?
	dirEntries, err := fs.stors.TransferRequestStor.ListDirectory(dir, transferRequest)

	inDir := &mcmodel.File{Path: projPath.ProjectPath, MimeType: "directory"}
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
	fmt.Printf("MCApi.Lookup: %q\n", path)
	projPath := projectpath.NewProjectPath(path)

	switch projPath.PathType {
	case projectpath.BadIDPath:
		return nil, fmt.Errorf("bad id path: %s", path)

	case projectpath.RootBasePath:
		// Return data on the root node
		fmt.Println("  Lookup RootBasePath")
		return nil, fmt.Errorf("root not supported")

	case projectpath.ProjectBasePath:
		// 	Return data on the project
		fmt.Println("  Lookup ProjectBasePath")
		return fs.lookupProject(path)

	case projectpath.UserBasePath:
		// Return data on the user
		fmt.Println("  Lookup UserBasePath")
		return fs.lookupUser(path)

	default:
		fmt.Println("  Lookup default")
		projPath := projectpath.NewProjectPath(path)
		f, err := fs.stors.FileStor.GetFileByPath(projPath.ProjectID, projPath.ProjectPath)
		return f, err
	}
}

func (fs *MCApi) lookupProject(path string) (*mcmodel.File, error) {
	projPath := projectpath.NewProjectPath(path)
	fmt.Println("lookupProject", path)

	transferRequests, err := fs.stors.TransferRequestStor.GetTransferRequestsForProject(projPath.ProjectID)
	switch {
	case err != nil:
		return nil, err

	case len(transferRequests) == 0:
		return nil, fmt.Errorf("no such path: %s", path)

	default:
		fmt.Printf("   found transferRequests %#v", transferRequests)
		// Found at least one transfer request for the project
		f := &mcmodel.File{
			Name:      fmt.Sprintf("%d", projPath.ProjectID),
			MimeType:  "directory",
			Path:      fmt.Sprintf("/%d", projPath.ProjectID),
			Directory: &mcmodel.File{Path: "/", Name: "/", MimeType: "directory"},
		}
		return f, nil
	}
}

func (fs *MCApi) lookupUser(path string) (*mcmodel.File, error) {
	projPath := projectpath.NewProjectPath(path)

	// If we are here then the project has been verified, so we need to make sure that the
	// user exists
	tr, err := fs.stors.TransferRequestStor.GetTransferRequestForProjectAndUser(projPath.ProjectID, projPath.UserID)
	if err != nil {
		return nil, err
	}

	if tr == nil {
		return nil, fmt.Errorf("no such active user %d for project %d", projPath.UserID, projPath.ProjectID)
	}

	f := &mcmodel.File{
		Name:     fmt.Sprintf("%d", projPath.UserID),
		MimeType: "directory",
		Path:     fmt.Sprintf("/%d/%d", projPath.ProjectID, projPath.UserID),
		Directory: &mcmodel.File{
			Path:     fmt.Sprintf("/%d", projPath.ProjectID),
			Name:     fmt.Sprintf("%d", projPath.ProjectID),
			MimeType: "directory",
		},
	}

	return f, nil
}

func (fs *MCApi) Mkdir(path string) (*mcmodel.File, error) {
	fmt.Println("MCApi.Mkdir path =", path)
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

func (fs *MCApi) FTruncate(path string) error {
	f := fs.knownFilesTracker.GetFile(path)
	if f == nil {
		fmt.Println("FTruncate - Unknown file", path)
	} else {
		fmt.Printf("FTruncate - Found file %s: %#v\n", path, f)
	}
	return nil
}

func (fs *MCApi) Open(path string, flags int) (f *mcmodel.File, isNewFile bool, err error) {
	fmt.Printf("MCApi Open %s\n", path)
	projPath := projectpath.NewProjectPath(path)
	f = fs.knownFilesTracker.GetFile(path)
	if f != nil {
		// Existing file found
		return f, false, nil
	}

	if flagSet(flags, syscall.O_RDONLY) {
		// If we are here then this is a request to **ONLY** open file for read. The file
		// needs to exist.
		f, err = fs.stors.FileStor.GetFileByPath(projPath.ProjectID, projPath.ProjectPath)
		return f, false, err
	}

	// If we are here then the file wasn't found in the list of already opened
	// files, so we need to create the file.
	f, err = fs.createNewFileVersion(projPath)
	if err != nil {
		fs.knownFilesTracker.Store(path, f)
	}

	return f, true, err
}

func (fs *MCApi) Release(path string, size uint64) error {
	knownFile := fs.knownFilesTracker.Get(path)
	if knownFile == nil {
		return syscall.ENOENT
	}

	projPath := projectpath.NewProjectPath(path)
	checksum := fmt.Sprintf("%x", knownFile.hasher.Sum(nil))
	err := fs.stors.TransferRequestStor.MarkFileReleased(knownFile.file, checksum, projPath.ProjectID, int64(size))

	// Add to convertible list after marking as released to prevent the condition where the
	// file hasn't been released but is picked up for conversion. This is a very unlikely
	// case, but easy to prevent by releasing then adding to conversions list.
	if knownFile.file.IsConvertible() {
		if _, err := fs.stors.ConversionStor.AddFileToConvert(knownFile.file); err != nil {
			slog.Error("Failed adding file to conversion", "file.ID", knownFile.file.ID)
		}
	}

	return err
}

func flagSet(flags, flagToCheck int) bool {
	return flags&flagToCheck == flagToCheck
}

// createNewFile will create a new mcmodel.File entry for the directory associated
// with the Node. It will create the directory where the file can be written to.
func (fs *MCApi) createNewFile(projPath *projectpath.ProjectPath) (*mcmodel.File, error) {
	dir, err := fs.stors.FileStor.GetDirByPath(projPath.ProjectID, filepath.Dir(projPath.ProjectPath))
	if err != nil {
		return nil, err
	}

	tr, err := fs.stors.TransferRequestStor.GetTransferRequestForProjectAndUser(projPath.ProjectID, projPath.UserID)
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

	tr, err := fs.stors.TransferRequestStor.GetTransferRequestForProjectAndUser(projPath.ProjectID, projPath.UserID)
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
