package mcfs

import (
	"crypto/md5"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/materials-commons/hydra/pkg/clog"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/fsstate"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/mcpath"
)

// LocalMCFSApi is the file system interface into Materials Commons. It has little knowledge of
// FUSE. It understands the Materials Commons calls to make to achieve FUSE file system
// operations, and returns the results in a way that the node can pass back.
type LocalMCFSApi struct {
	//
	stors                *stor.Stors
	transferStateTracker *fsstate.TransferStateTracker
	pathParser           mcpath.Parser
	mcfsRoot             string
}

func NewLocalMCFSApi(stors *stor.Stors, tracker *fsstate.TransferStateTracker, pathParser mcpath.Parser, mcfsRoot string) *LocalMCFSApi {
	return &LocalMCFSApi{
		stors:                stors,
		transferStateTracker: tracker,
		mcfsRoot:             mcfsRoot,
		pathParser:           pathParser,
	}
}

func (fsapi *LocalMCFSApi) Create(path string) (*mcmodel.File, error) {
	parsedPath, _ := fsapi.pathParser.Parse(path)
	clog.UsingCtx(parsedPath.TransferKey()).Debugf("fsapi.Create %s = %s, %s\n", path, parsedPath.TransferKey(), parsedPath.ProjectPath())
	if file := fsapi.transferStateTracker.GetFile(parsedPath.TransferKey(), parsedPath.ProjectPath()); file != nil {
		// This should not happen - Create was called on a file that the file
		// system is already tracking as opened.
		return nil, fmt.Errorf("file found on create: %s", path)
	}

	f, err := fsapi.createNewFile(parsedPath)
	fsapi.transferStateTracker.Store(parsedPath.TransferKey(), parsedPath.ProjectPath(), f, fsstate.FileStateOpen)

	return f, err
}

// createNewFile will create a new mcmodel.File entry for the directory associated
// with the Node. It will create the directory where the file can be written to.
func (fsapi *LocalMCFSApi) createNewFile(p mcpath.Path) (*mcmodel.File, error) {
	dir, err := fsapi.stors.FileStor.GetDirByPath(p.ProjectID(), filepath.Dir(p.ProjectPath()))
	if err != nil {
		return nil, err
	}

	tr, err := fsapi.stors.TransferRequestStor.GetTransferRequestForProjectAndUser(p.ProjectID(), p.UserID())
	if err != nil {
		return nil, err
	}

	name := filepath.Base(p.ProjectPath())

	file := &mcmodel.File{
		ProjectID:   p.ProjectID(),
		Name:        name,
		DirectoryID: dir.ID,
		Size:        0,
		Checksum:    "",
		MimeType:    determineMimeType(name),
		OwnerID:     p.UserID(),
		Current:     false,
	}

	return fsapi.stors.TransferRequestStor.CreateNewFile(file, dir, tr)
}

// Open will open a file. It will create a new version if opening a new file for write. An open in Append
// mode should only happen after a new file has been created (ie, a TransferRequestFile created in the database).
func (fsapi *LocalMCFSApi) Open(path string, flags int) (f *mcmodel.File, isNewFile bool, err error) {
	parsedPath, err := fsapi.pathParser.Parse(path)
	if err != nil {
		clog.Global().Debugf("LocalMCFSApi Open failed on pathParser.Parse %s/%v: %s", path, flags, err)
		return nil, false, err
	}

	key := parsedPath.TransferKey()
	clog.UsingCtx(key).Debugf("LocalMCFSApi Open %s/%v", path, flags)

	switch {
	case isReadonly(flags):
		return fsapi.openReadonly(path, flags, parsedPath)
	case flagSet(flags, syscall.O_APPEND):
		return fsapi.openForAppend(path, flags, parsedPath)
	default:
		return fsapi.openForWrite(path, flags, parsedPath)
	}
}

// openReadonly handles requests for opens that will only read the file.
func (fsapi *LocalMCFSApi) openReadonly(path string, _ int, parsedPath mcpath.Path) (f *mcmodel.File, isNewFile bool, err error) {
	key := parsedPath.TransferKey()
	clog.UsingCtx(key).Debugf("LocalMCFSApi Open %s readonly", path)
	f, err = fsapi.stors.FileStor.GetFileByPath(parsedPath.ProjectID(), parsedPath.ProjectPath())

	return f, false, err
}

// openForAppend means the file is being opened with the O_APPEND mode set. It will reconstruct the file state, including
// the hash if the internal state is missing.
func (fsapi *LocalMCFSApi) openForAppend(path string, flags int, parsedPath mcpath.Path) (f *mcmodel.File, isNewFile bool, err error) {
	key := parsedPath.TransferKey()
	clog.UsingCtx(key).Debugf("LocalMCFSApi Open %s withAppend", path)

	// Attempt to find the state and reset the hash.
	f = fsapi.transferStateTracker.GetFile(key, parsedPath.ProjectPath())
	if f != nil {
		// We found an existing state, so there is no recovery that needs to be done.
		return f, false, nil
	}

	// If we are here then an attempt was made to open an existing file in append mode, but there is
	// no state be tracked. This happens when the file system has been restarted. There should be an
	// existing TransferRequestFile. We can look that up and rebuild the state. Since this being opened
	// in append mode we will also need to recompute the hash up to this point. For large files this
	// could take a bit of time to re-read the whole file and compute the hash over the existing bytes.

	transferRequest := parsedPath.TransferRequest()
	projPath := parsedPath.ProjectPath()
	transferRequestFile, err := fsapi.stors.TransferRequestFileStor.GetTransferRequestFileByPathForTransferRequest(projPath, transferRequest)
	if err != nil {
		// A request was made to open a file and there is no TransferRequestFile. This shouldn't happen!!
		clog.UsingCtx(key).Errorf("Open withAppend on path %s, no TransferRequestFile found: %s", path, err)
		return nil, false, err
	}

	// Found an existing transferRequestFile, so lets build out new state to track it.

	// Create the state
	_, state := fsapi.transferStateTracker.Store(key, projPath, transferRequestFile.File, fsstate.FileStateOpen)

	// Recompute hash using the hasher that was created when the new state was created
	fh, err := os.Open(transferRequestFile.File.ToUnderlyingFilePath(fsapi.mcfsRoot))
	if err != nil {
		// This shouldn't happen. If it does we should probably remove the state...
		// TODO: What to do with state when this error occurs.
		return nil, false, err
	}

	defer fh.Close()
	_, _ = io.Copy(state.Hasher, fh)

	return transferRequestFile.File, false, nil
}

// openForWrite opens a file in O_WRONLY or O_RDWR mode. It checks if there is existing state and resets the hash
// if there is. This happens because a write mode is likely overwriting file contents. If this is the first time
// the file is being opened then a new version will be created.
func (fsapi *LocalMCFSApi) openForWrite(path string, flags int, parsedPath mcpath.Path) (f *mcmodel.File, isNewFile bool, err error) {
	key := parsedPath.TransferKey()
	clog.UsingCtx(key).Debugf("LocalMCFSApi Open %s withAppend", path)

	// Request to open the file for write, but not append. In this case we reset
	// the hash state.
	f = fsapi.transferStateTracker.GetFileWithHashReset(key, parsedPath.ProjectPath())
	if f != nil {
		// We found an existing state, so there is no recovery that needs to be done.
		return f, false, nil
	}

	// If we are here then there was no transfer state. So, lets see if we can
	// find one. It's possible there is no TransferRequestFile. This happens
	// when an open for write request is made for an existing project file. In
	// that case we need to create a new version.

	// First lets do the simple case and check if there is an existing TransferRequestFile.
	transferRequest := parsedPath.TransferRequest()
	projPath := parsedPath.ProjectPath()
	transferRequestFile, err := fsapi.stors.TransferRequestFileStor.GetTransferRequestFileByPathForTransferRequest(projPath, transferRequest)
	if err == nil {
		// Found a transferRequestFile. So let's create a new state for it and then use the mcmodel.File that
		// is referenced by the TransferRequestFile.
		fsapi.transferStateTracker.Store(key, projPath, transferRequestFile.File, fsstate.FileStateOpen)
		return transferRequestFile.File, false, nil
	}

	// No TransferRequestFile, so we need to create a new file version.
	clog.UsingCtx(key).Debugf("LocalMCFSApi Open %s create new file version", path)
	f, err = fsapi.createNewFileVersion(parsedPath)
	if err != nil {
		fsapi.transferStateTracker.Store(parsedPath.TransferKey(), parsedPath.ProjectPath(), f, fsstate.FileStateOpen)
	}

	return f, true, err
}

// createNewFileVersion creates a new file version if there isn't already a version of the file
// associated with this transfer request instance. It checks the transferStateTracker to determine
// if a new version has already been created. If a new version was already created then it will return
// that version. Otherwise, it will create a new version and add it to the OpenedFilesTracker. In
// addition, when a new version is created, the associated on disk directory is created.
func (fsapi *LocalMCFSApi) createNewFileVersion(p mcpath.Path) (*mcmodel.File, error) {
	var err error

	name := filepath.Base(p.ProjectPath())

	dir, err := fsapi.stors.FileStor.GetDirByPath(p.ProjectID(), filepath.Dir(p.ProjectPath()))
	if err != nil {
		return nil, err
	}

	tr, err := fsapi.stors.TransferRequestStor.GetTransferRequestForProjectAndUser(p.ProjectID(), p.UserID())
	if err != nil {
		return nil, err
	}

	// There isn't an existing upload, so create a new one
	f := &mcmodel.File{
		ProjectID:   p.ProjectID(),
		Name:        name,
		DirectoryID: dir.ID,
		Size:        0,
		Checksum:    "",
		MimeType:    determineMimeType(name),
		OwnerID:     p.UserID(),
		Current:     false,
	}

	f, err = fsapi.stors.TransferRequestStor.CreateNewFile(f, dir, tr)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func (fsapi *LocalMCFSApi) Release(path string, size uint64) error {
	parsedPath, _ := fsapi.pathParser.Parse(path)
	key := parsedPath.TransferKey()
	fileState := fsapi.transferStateTracker.Get(key, parsedPath.ProjectPath())
	if fileState == nil {
		clog.UsingCtx(key).Errorf("LocalMCFSApi.Release fileState is nil for %s\n", path)
		return syscall.ENOENT
	}

	fileState.FileState = fsstate.FileStateClosed
	checksum := ""
	var err error
	if fileState.HashInvalid {
		fsapi.computeAndUpdateChecksum(path, fileState, size)
	} else {
		checksum = fmt.Sprintf("%x", fileState.Hasher.Sum(nil))
		err = fsapi.stors.TransferRequestStor.MarkFileReleased(fileState.File, checksum, parsedPath.ProjectID(), int64(size))
		if err != nil {
			clog.UsingCtx(key).Debugf("LocalMCFSApi.Release MarkFileReleased failed with err %s\n", err)
			return err
		}
		// Add to convertible list after marking as released to prevent the condition where the
		// file hasn't been released but is picked up for conversion. This is a very unlikely
		// case, but easy to prevent by releasing then adding to conversions list.
		if fileState.File.IsConvertible() {
			if _, err := fsapi.stors.ConversionStor.AddFileToConvert(fileState.File); err != nil {
				clog.UsingCtx(key).Errorf("Failed adding file to conversion %d", fileState.File.ID)
			}
		}
	}

	return nil
}

// computeAndUpdateChecksum will recompute a files checksum when the current state of the checksum is marked as
// invalid. An invalid state occurs when the filesystem no longer knows the file state. This happens when using
// truncate or seek, as the state of the hash was built up over the previous states.
//
// This function will create a new Hasher in the fileState and then read the existing file to determine the hash.
// It then set HashInvalid to false, so that new writes (appends) to the file can use the existing state. Note
// that for
func (fsapi *LocalMCFSApi) computeAndUpdateChecksum(path string, fileState *fsstate.AccessedFileState, size uint64) {
	fileState.Hasher = md5.New()
	f := fileState.File

	fh, err := os.Open(f.ToUnderlyingFilePath(fsapi.mcfsRoot))
	if err != nil {
		// log that we couldn't compute the hash
		return
	}

	defer fh.Close()

	_, _ = io.Copy(fileState.Hasher, fh)
	checksum := fmt.Sprintf("%x", fileState.Hasher.Sum(nil))

	parsedPath, _ := fsapi.pathParser.Parse(path)

	fsapi.transferStateTracker.WithLockHeld(parsedPath.TransferKey(), parsedPath.ProjectPath(), func(fileState *fsstate.AccessedFileState) {
		fileState.HashInvalid = false
		if err := fsapi.stors.FileStor.UpdateMetadataForFileAndProject(f, checksum, int64(size)); err != nil {
			// log that we couldn't update the database
			return
		}
	})
}

func (fsapi *LocalMCFSApi) Lookup(path string) (*mcmodel.File, error) {
	parsedPath, _ := fsapi.pathParser.Parse(path)
	clog.UsingCtx(parsedPath.TransferKey()).Debugf("LocalMCFSApi.Lookup %s", path)
	return parsedPath.Lookup()
}

func (fsapi *LocalMCFSApi) Readdir(path string) ([]mcmodel.File, error) {
	parsedPath, _ := fsapi.pathParser.Parse(path)
	clog.UsingCtx(parsedPath.TransferKey()).Debugf("LocalMCFSApi.Readdir %s", path)
	return parsedPath.List()
}

func (fsapi *LocalMCFSApi) Mkdir(path string) (*mcmodel.File, error) {
	clog.Global().Debugf("LocalMCFSApi.Mkdir %s", path)
	parsedPath, _ := fsapi.pathParser.Parse(path)
	key := parsedPath.TransferKey()
	clog.UsingCtx(key).Debugf("LocalMCFSApi.Mkdir GetFileByPath(%d, '%s')\n", parsedPath.ProjectID(), filepath.Dir(parsedPath.ProjectPath()))
	parentDir, err := fsapi.stors.FileStor.GetFileByPath(parsedPath.ProjectID(), filepath.Dir(parsedPath.ProjectPath()))
	if err != nil {
		return nil, err
	}

	return fsapi.stors.FileStor.CreateDirectory(parentDir.ID, parsedPath.ProjectID(), parsedPath.UserID(),
		parsedPath.ProjectPath(), filepath.Base(parsedPath.ProjectPath()))
}

func (fsapi *LocalMCFSApi) GetRealPath(path string) (realpath string, err error) {
	parsedPath, _ := fsapi.pathParser.Parse(path)
	if file := fsapi.transferStateTracker.GetFile(parsedPath.TransferKey(), parsedPath.ProjectPath()); file != nil {
		// Found known file, so return it's real path
		return file.ToUnderlyingFilePath(fsapi.mcfsRoot), nil
	}

	// Didn't find a previously opened file, so look up file.
	file, err := fsapi.stors.FileStor.GetFileByPath(parsedPath.ProjectID(), parsedPath.ProjectPath())
	if err != nil {
		return "", err
	}

	return file.ToUnderlyingFilePath(fsapi.mcfsRoot), nil
}

func (fsapi *LocalMCFSApi) GetKnownFileRealPath(path string) (string, error) {
	parsedPath, _ := fsapi.pathParser.Parse(path)
	f := fsapi.transferStateTracker.GetFile(parsedPath.TransferKey(), parsedPath.ProjectPath())
	if f != nil {
		return f.ToUnderlyingFilePath(fsapi.mcfsRoot), nil
	}

	return "", fmt.Errorf("unknown file: %s", path)
}

func (fsapi *LocalMCFSApi) FTruncate(path string, size uint64) (error, *syscall.Stat_t) {
	parsedPath, _ := fsapi.pathParser.Parse(path)
	f := fsapi.transferStateTracker.GetFile(parsedPath.TransferKey(), parsedPath.ProjectPath())
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

func isReadonly(flags int) bool {
	switch {
	case flagSet(flags, syscall.O_WRONLY):
		return false
	case flagSet(flags, syscall.O_RDWR):
		return false
	default:
		// For open one of O_WRONLY, O_RDWR or O_RDONLY must be set. If
		// we are here then neither O_WRONLY nor O_RDWR was set, so O_RDONLY
		// must be true.
		return true
	}
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
