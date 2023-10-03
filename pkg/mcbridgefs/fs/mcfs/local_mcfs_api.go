package mcfs

import (
	"crypto/md5"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/materials-commons/hydra/pkg/mcbridgefs/fs/mcfs/mcpath"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

// LocalMCFSApi is the file system interface into Materials Commons. It has little knowledge of
// FUSE. It understands the Materials Commons calls to make to achieve FUSE file system
// operations, and returns the results in a way that the node can pass back.
type LocalMCFSApi struct {
	//
	stors             *stor.Stors
	knownFilesTracker *KnownFilesTracker
	mcfsRoot          string
}

func NewLocalMCFSApi(stors *stor.Stors, tracker *KnownFilesTracker, mcfsRoot string) *LocalMCFSApi {
	return &LocalMCFSApi{
		stors:             stors,
		knownFilesTracker: tracker,
		mcfsRoot:          mcfsRoot,
	}
}

func (fsapi *LocalMCFSApi) Create(path string) (*mcmodel.File, error) {
	projPath := makeProjectPath(path)
	if file := fsapi.knownFilesTracker.GetFile(projPath.FullPath()); file != nil {
		// This should not happen - Create was called on a file that the file
		// system is already tracking as opened.
		return nil, fmt.Errorf("file found on create: %s", path)
	}

	f, err := fsapi.createNewFile(projPath)
	fsapi.knownFilesTracker.Store(path, f)

	return f, err
}

func (fsapi *LocalMCFSApi) Open(path string, flags int) (f *mcmodel.File, isNewFile bool, err error) {
	slog.Debug("LocalMCFSApi Open", "path", path)
	projPath := makeProjectPath(path)
	f = fsapi.knownFilesTracker.GetFile(path)
	if f != nil {
		// Existing file found
		return f, false, nil
	}

	if flagSet(flags, syscall.O_RDONLY) {
		// If we are here then this is a request to **ONLY** open file for read. The file
		// needs to exist.
		f, err = fsapi.stors.FileStor.GetFileByPath(projPath.ProjectID(), projPath.ProjectPath())
		return f, false, err
	}

	// If we are here then the file wasn't found in the list of already opened
	// files, so we need to create the file.
	f, err = fsapi.createNewFileVersion(projPath)
	if err != nil {
		fsapi.knownFilesTracker.Store(path, f)
	}

	return f, true, err
}

// createNewFile will create a new mcmodel.File entry for the directory associated
// with the Node. It will create the directory where the file can be written to.
func (fsapi *LocalMCFSApi) createNewFile(projPath *mcpath.ProjectPath) (*mcmodel.File, error) {
	dir, err := fsapi.stors.FileStor.GetDirByPath(projPath.ProjectID(), filepath.Dir(projPath.ProjectPath()))
	if err != nil {
		return nil, err
	}

	tr, err := fsapi.stors.TransferRequestStor.GetTransferRequestForProjectAndUser(projPath.ProjectID(), projPath.UserID())
	if err != nil {
		return nil, err
	}

	name := filepath.Base(projPath.ProjectPath())

	file := &mcmodel.File{
		ProjectID:   projPath.ProjectID(),
		Name:        name,
		DirectoryID: dir.ID,
		Size:        0,
		Checksum:    "",
		MimeType:    determineMimeType(name),
		OwnerID:     projPath.UserID(),
		Current:     false,
	}

	return fsapi.stors.TransferRequestStor.CreateNewFile(file, dir, tr)
}

// createNewFileVersion creates a new file version if there isn't already a version of the file
// associated with this transfer request instance. It checks the knownFilesTracker to determine
// if a new version has already been created. If a new version was already created then it will return
// that version. Otherwise, it will create a new version and add it to the OpenedFilesTracker. In
// addition, when a new version is created, the associated on disk directory is created.
func (fsapi *LocalMCFSApi) createNewFileVersion(projPath *mcpath.ProjectPath) (*mcmodel.File, error) {
	var err error

	name := filepath.Base(projPath.ProjectPath())

	dir, err := fsapi.stors.FileStor.GetDirByPath(projPath.ProjectID(), filepath.Dir(projPath.ProjectPath()))
	if err != nil {
		return nil, err
	}

	tr, err := fsapi.stors.TransferRequestStor.GetTransferRequestForProjectAndUser(projPath.ProjectID(), projPath.UserID())
	if err != nil {
		return nil, err
	}

	// There isn't an existing upload, so create a new one
	f := &mcmodel.File{
		ProjectID:   projPath.ProjectID(),
		Name:        name,
		DirectoryID: dir.ID,
		Size:        0,
		Checksum:    "",
		MimeType:    determineMimeType(name),
		OwnerID:     projPath.UserID(),
		Current:     false,
	}

	f, err = fsapi.stors.TransferRequestStor.CreateNewFile(f, dir, tr)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func (fsapi *LocalMCFSApi) Release(path string, size uint64) error {
	knownFile := fsapi.knownFilesTracker.Get(path)
	if knownFile == nil {
		fmt.Printf("LocalMCFSApi.Release knownFile is nil for %s\n", path)
		return syscall.ENOENT
	}

	projPath := makeProjectPath(path)
	checksum := ""
	var err error
	if knownFile.hashInvalid {
		var sequence int
		fsapi.knownFilesTracker.WithLockHeld(path, func(knownFile *KnownFile) {
			knownFile.sequence = knownFile.sequence + 1
			sequence = knownFile.sequence
		})
		go fsapi.computeAndUpdateChecksum(path, knownFile.file, size, sequence)
	} else {
		checksum = fmt.Sprintf("%x", knownFile.hasher.Sum(nil))
		err = fsapi.stors.TransferRequestStor.MarkFileReleased(knownFile.file, checksum, projPath.ProjectID(), int64(size))
		// Add to convertible list after marking as released to prevent the condition where the
		// file hasn't been released but is picked up for conversion. This is a very unlikely
		// case, but easy to prevent by releasing then adding to conversions list.
		if knownFile.file.IsConvertible() {
			if _, err := fsapi.stors.ConversionStor.AddFileToConvert(knownFile.file); err != nil {
				slog.Error("Failed adding file to conversion", "file.ID", knownFile.file.ID)
			}
		}

		if err != nil {
			fmt.Printf("LocalMCFSApi.Release MarkFileReleased failed with err %s\n", err)
		}
		return err
	}

	return nil
}

func (fsapi *LocalMCFSApi) computeAndUpdateChecksum(path string, f *mcmodel.File, size uint64, sequence int) {
	hasher := md5.New()
	fh, err := os.Open(f.ToUnderlyingFilePath(fsapi.mcfsRoot))
	if err != nil {
		// log that we couldn't compute the hash
		return
	}

	_, _ = io.Copy(hasher, fh)
	checksum := fmt.Sprintf("%x", hasher.Sum(nil))

	fsapi.knownFilesTracker.WithLockHeld(path, func(knownFile *KnownFile) {
		if knownFile.sequence == sequence {
			// This check ensures that another thread wasn't kicked off to compute the checksum. This could
			// happen if the file was closed with an invalid checksum, a thread was kicked off, and then while
			// the thread was computing the checksum, another open/close happened that kicked off another
			// checksum computation. If the sequence is equal, then this is the thread that needs to update
			// the checksum and size.
			if err := fsapi.stors.FileStor.UpdateMetadataForFileAndProject(f, checksum, int64(size)); err != nil {
				// log that we couldn't update the database
				return
			}
		}
	})
}

func (fsapi *LocalMCFSApi) Lookup(path string) (*mcmodel.File, error) {
	slog.Debug("LocalMCFSApi.Lookup", "path", path)
	projPath := makeProjectPath(path)

	switch projPath.PathType() {
	case mcpath.BadIDPath:
		return nil, fmt.Errorf("bad id path: %s", path)

	case mcpath.RootBasePath:
		// Return data on the root node
		return nil, fmt.Errorf("root not supported")

	case mcpath.ProjectBasePath:
		// 	Return data on the project
		return fsapi.lookupProject(path)

	case mcpath.UserBasePath:
		// Return data on the user
		return fsapi.lookupUser(path)

	default:
		projPath := makeProjectPath(path)
		f, err := fsapi.stors.FileStor.GetFileByPath(projPath.ProjectID(), projPath.ProjectPath())
		return f, err
	}
}

func (fsapi *LocalMCFSApi) lookupProject(path string) (*mcmodel.File, error) {
	projPath := makeProjectPath(path)

	transferRequests, err := fsapi.stors.TransferRequestStor.GetTransferRequestsForProject(projPath.ProjectID())
	switch {
	case err != nil:
		return nil, err

	case len(transferRequests) == 0:
		return nil, fmt.Errorf("no such path: %s", path)

	default:
		// Found at least one transfer request for the project
		f := &mcmodel.File{
			Name:      fmt.Sprintf("%d", projPath.ProjectID()),
			MimeType:  "directory",
			Path:      fmt.Sprintf("/%d", projPath.ProjectID()),
			Directory: &mcmodel.File{Path: "/", Name: "/", MimeType: "directory"},
		}
		return f, nil
	}
}

func (fsapi *LocalMCFSApi) lookupUser(path string) (*mcmodel.File, error) {
	projPath := makeProjectPath(path)

	// If we are here then the project has been verified, so we need to make sure that the
	// user exists
	tr, err := fsapi.stors.TransferRequestStor.GetTransferRequestForProjectAndUser(projPath.ProjectID(), projPath.UserID())
	if err != nil {
		return nil, err
	}

	if tr == nil {
		return nil, fmt.Errorf("no such active user %d for project %d", projPath.UserID(), projPath.ProjectID())
	}

	f := &mcmodel.File{
		Name:     fmt.Sprintf("%d", projPath.UserID()),
		MimeType: "directory",
		Path:     fmt.Sprintf("/%d/%d", projPath.ProjectID(), projPath.UserID()),
		Directory: &mcmodel.File{
			Path:     fmt.Sprintf("/%d", projPath.ProjectID()),
			Name:     fmt.Sprintf("%d", projPath.ProjectID()),
			MimeType: "directory",
		},
	}

	return f, nil
}

func (fsapi *LocalMCFSApi) Readdir(path string) ([]mcmodel.File, error) {
	slog.Debug("LocalMCFSApi.Readdir", "path", path)

	projPath := makeProjectPath(path)

	switch projPath.PathType() {
	case mcpath.BadIDPath:
		return nil, fmt.Errorf("bad id path: %s", path)
	case mcpath.RootBasePath:
		// Return the list of projects that have transfer requests
		return fsapi.listActiveProjects()
	case mcpath.ProjectBasePath:
		// Return the list of users that have transfer requests for this project
		return fsapi.listActiveUsersForProject(path)
	default:
		// Return directory contents for that /project/user/rest/of/project/path
		return fsapi.listProjectDirectory(path)
	}
}

func (fsapi *LocalMCFSApi) listActiveProjects() ([]mcmodel.File, error) {
	transferRequests, err := fsapi.stors.TransferRequestStor.ListTransferRequests()
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

func (fsapi *LocalMCFSApi) listActiveUsersForProject(path string) ([]mcmodel.File, error) {
	projPath := makeProjectPath(path)
	transferRequests, err := fsapi.stors.TransferRequestStor.ListTransferRequests()
	if err != nil {
		return nil, err
	}

	inDir := &mcmodel.File{Path: fmt.Sprintf("/%d", projPath.ProjectID()), MimeType: "directory"}
	var dirEntries []mcmodel.File

	for _, tr := range transferRequests {
		if tr.ProjectID == projPath.ProjectID() {
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
		return nil, fmt.Errorf("no such project: %d", projPath.ProjectID())
	}

	return dirEntries, nil
}

func (fsapi *LocalMCFSApi) listProjectDirectory(path string) ([]mcmodel.File, error) {
	projPath := makeProjectPath(path)

	dir, err := fsapi.stors.FileStor.GetDirByPath(projPath.ProjectID(), projPath.ProjectPath())
	if err != nil {
		return nil, err
	}

	transferRequest, err := fsapi.stors.TransferRequestStor.
		GetTransferRequestForProjectAndUser(projPath.ProjectID(), projPath.UserID())
	if err != nil {
		return nil, err
	}

	// Make list directory to a pointer for transferRequest?
	dirEntries, err := fsapi.stors.TransferRequestStor.ListDirectory(dir, transferRequest)

	inDir := &mcmodel.File{Path: projPath.ProjectPath(), MimeType: "directory"}
	for _, entry := range dirEntries {
		entry.Directory = inDir
	}

	return dirEntries, nil
}

func (fsapi *LocalMCFSApi) Mkdir(path string) (*mcmodel.File, error) {
	slog.Debug("LocalMCFSApi.Mkdir", "path", path)
	projPath := makeProjectPath(path)
	parentDir, err := fsapi.stors.FileStor.GetFileByPath(projPath.ProjectID(), filepath.Dir(projPath.ProjectPath()))
	if err != nil {
		return nil, err
	}

	return fsapi.stors.FileStor.CreateDirectory(parentDir.ID, projPath.ProjectID(), projPath.UserID(),
		projPath.ProjectPath(), filepath.Base(projPath.ProjectPath()))
}

func (fsapi *LocalMCFSApi) GetRealPath(path string) (realpath string, err error) {
	if file := fsapi.knownFilesTracker.GetFile(path); file != nil {
		// Found known file, so return it's real path
		return file.ToUnderlyingFilePath(fsapi.mcfsRoot), nil
	}

	// Didn't find a previously opened file, so look up file.
	projPath := makeProjectPath(path)
	file, err := fsapi.stors.FileStor.GetFileByPath(projPath.ProjectID(), projPath.ProjectPath())
	if err != nil {
		return "", err
	}

	return file.ToUnderlyingFilePath(fsapi.mcfsRoot), nil
}

func (fsapi *LocalMCFSApi) GetKnownFileRealPath(path string) (string, error) {
	f := fsapi.knownFilesTracker.GetFile(path)
	if f != nil {
		return f.ToUnderlyingFilePath(fsapi.mcfsRoot), nil
	}

	return "", fmt.Errorf("unknown file: %s", path)
}

func (fsapi *LocalMCFSApi) FTruncate(path string, size uint64) (error, *syscall.Stat_t) {
	f := fsapi.knownFilesTracker.GetFile(path)
	if f == nil {
		return syscall.ENOENT, nil
	}

	if err := syscall.Truncate(f.ToUnderlyingFilePath(fsapi.mcfsRoot), int64(size)); err != nil {
		return fs.ToErrno(err), nil
	}

	st := syscall.Stat_t{}
	if err := syscall.Lstat(f.ToUnderlyingFilePath(fsapi.mcfsRoot), &st); err != nil {
		return fs.ToErrno(err), nil
	}

	return nil, &st
}

func flagSet(flags, flagToCheck int) bool {
	return flags&flagToCheck == flagToCheck
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

func makeProjectPath(path string) *mcpath.ProjectPath {
	pathParser := mcpath.NewProjectPathParser()
	p, _ := pathParser.Parse(path)
	return p.(*mcpath.ProjectPath)
}
