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

	"github.com/apex/log"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/mcpath"
)

// LocalMCFSApi is the file system interface into Materials Commons. It has little knowledge of
// FUSE. It understands the Materials Commons calls to make to achieve FUSE file system
// operations, and returns the results in a way that the node can pass back.
type LocalMCFSApi struct {
	//
	stors                *stor.Stors
	transferStateTracker *TransferStateTracker
	pathParser           mcpath.Parser
	mcfsRoot             string
}

func NewLocalMCFSApi(stors *stor.Stors, tracker *TransferStateTracker, pathParser mcpath.Parser, mcfsRoot string) *LocalMCFSApi {
	return &LocalMCFSApi{
		stors:                stors,
		transferStateTracker: tracker,
		mcfsRoot:             mcfsRoot,
		pathParser:           pathParser,
	}
}

func (fsapi *LocalMCFSApi) Create(path string) (*mcmodel.File, error) {
	parsedPath, _ := fsapi.pathParser.Parse(path)
	log.Debugf("fsapi.Create %s = %s, %s\n", path, parsedPath.TransferKey(), parsedPath.ProjectPath())
	if file := fsapi.transferStateTracker.GetFile(parsedPath.TransferKey(), parsedPath.ProjectPath()); file != nil {
		// This should not happen - Create was called on a file that the file
		// system is already tracking as opened.
		return nil, fmt.Errorf("file found on create: %s", path)
	}

	f, err := fsapi.createNewFile(parsedPath)
	fsapi.transferStateTracker.Store(parsedPath.TransferKey(), parsedPath.ProjectPath(), f, FileStateOpen)

	return f, err
}

func (fsapi *LocalMCFSApi) Open(path string, flags int) (f *mcmodel.File, isNewFile bool, err error) {
	log.Debugf("LocalMCFSApi Open %s", path)
	parsedPath, _ := fsapi.pathParser.Parse(path)
	f = fsapi.transferStateTracker.GetFile(parsedPath.TransferKey(), parsedPath.ProjectPath())
	if f != nil {
		// Existing file found
		return f, false, nil
	}

	if flagSet(flags, syscall.O_RDONLY) {
		// If we are here then this is a request to **ONLY** open file for read. The file
		// needs to exist.
		f, err = fsapi.stors.FileStor.GetFileByPath(parsedPath.ProjectID(), parsedPath.ProjectPath())
		return f, false, err
	}

	// If we are here then the file wasn't found in the list of already opened
	// files, so we need to create the file.
	f, err = fsapi.createNewFileVersion(parsedPath)
	if err != nil {
		fsapi.transferStateTracker.Store(parsedPath.TransferKey(), parsedPath.ProjectPath(), f, FileStateOpen)
	}

	return f, true, err
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
	fileState := fsapi.transferStateTracker.Get(parsedPath.TransferKey(), parsedPath.ProjectPath())
	if fileState == nil {
		fmt.Printf("LocalMCFSApi.Release fileState is nil for %s\n", path)
		return syscall.ENOENT
	}

	fileState.FileState = FileStateClosed
	checksum := ""
	var err error
	if fileState.HashInvalid {
		var sequence int
		fileState.Sequence = fileState.Sequence + 1
		sequence = fileState.Sequence
		go fsapi.computeAndUpdateChecksum(path, fileState.File, size, sequence)
	} else {
		checksum = fmt.Sprintf("%x", fileState.Hasher.Sum(nil))
		err = fsapi.stors.TransferRequestStor.MarkFileReleased(fileState.File, checksum, parsedPath.ProjectID(), int64(size))
		if err != nil {
			log.Debugf("LocalMCFSApi.Release MarkFileReleased failed with err %s\n", err)
			return err
		}
		// Add to convertible list after marking as released to prevent the condition where the
		// file hasn't been released but is picked up for conversion. This is a very unlikely
		// case, but easy to prevent by releasing then adding to conversions list.
		if fileState.File.IsConvertible() {
			if _, err := fsapi.stors.ConversionStor.AddFileToConvert(fileState.File); err != nil {
				log.Errorf("Failed adding file to conversion %d", fileState.File.ID)
			}
		}
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

	parsedPath, _ := fsapi.pathParser.Parse(path)

	fsapi.transferStateTracker.WithLockHeld(parsedPath.TransferKey(), parsedPath.ProjectPath(), func(fileState *AccessedFileState) {
		if fileState.Sequence == sequence {
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
	log.Debugf("LocalMCFSApi.Lookup %s", path)
	parsedPath, _ := fsapi.pathParser.Parse(path)
	return parsedPath.Lookup()
}

func (fsapi *LocalMCFSApi) Readdir(path string) ([]mcmodel.File, error) {
	log.Debugf("LocalMCFSApi.Readdir %s", path)
	parsedPath, _ := fsapi.pathParser.Parse(path)
	return parsedPath.List()
}

func (fsapi *LocalMCFSApi) Mkdir(path string) (*mcmodel.File, error) {
	log.Debugf("LocalMCFSApi.Mkdir %s", path)
	parsedPath, _ := fsapi.pathParser.Parse(path)
	log.Debugf("LocalMCFSApi.Mkdir GetFileByPath(%d, '%s')\n", parsedPath.ProjectID(), filepath.Dir(parsedPath.ProjectPath()))
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
