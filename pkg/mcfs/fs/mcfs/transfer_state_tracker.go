package mcfs

import (
	"crypto/md5"
	"hash"
	"sync"

	"github.com/materials-commons/hydra/pkg/globus"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

// The TransferStateTracker is used to track files the filesystem has already seen. The
// paths that are stored include the user/project combination. This means that there
// could be multiple instances of the same project file path, but different for
// each user. This is ok, because each these will represent a different file version
// because each user will get their own version when opened for write.
//
// Note: Files are only stored if opened for write. That means if a file is opened
// for write, then later opened for read, it will refer to the version opened for write.
// All calls on a TransferStateTracker are thread safe.
type TransferStateTracker struct {
	// Synchronizes access to the TransferRequestStates map
	mu sync.Mutex

	// A map of known files. This is a two level map. The first key is a unique tied to an entire
	// upload instance. For example a transfer request uuid or a project/user combination. The
	// second map is keyed by the path for the project.
	TransferRequestStates map[string]*TransferRequestState
}

type GlobusTask struct {
	Task      globus.Task
	Transfers []globus.Transfer
}

type TransferRequestState struct {
	GlobusTasks        []GlobusTask
	AccessedFileStates map[string]*AccessedFileState
}

const FileStateOpen = "open"
const FileStateClosed = "closed"

// AccessedFileState tracks the state of a file that was opened for write. This tracks the underlying
// database file entry and the hasher used to construct the checksum.
type AccessedFileState struct {
	// File is the database file the file system file is associated with
	File *mcmodel.File

	// Hasher is used to create the checksum. As writes are done to a file the
	// checksum is updated.
	Hasher hash.Hash

	// HashInvalid is set to true when a user seeks or truncates a file. When
	// that happens the hash state is invalid.
	HashInvalid bool

	FileState string

	// Sequence is used when hashInvalid is set to true. The sequence is used by
	// the thread that is recomputing a file hash by reading the entire file.
	// It's possible that a process could seek into a file, causing hashInvalid
	// to be set to true. Then close the file, which causes a thread to launch
	// to compute the checksum. Then while the thread is computing the checksum
	// it could reopen the file and start writing to it again, then close it
	// which would cause a second thread to launch to compute the checksum. The
	// sequence is used to determine if a thread should update the checksum. If
	// a thread completes computing the checksum but finds that the sequence has
	// changed from the one passed to the thread then it knows another thread is
	// computing a more up-to-date checksum. Only the thread that has a sequence
	// matching the sequence in the AccessedFileState entry will update the checksum.
	Sequence int
}

func NewTransferStateTracker() *TransferStateTracker {
	return &TransferStateTracker{TransferRequestStates: make(map[string]*TransferRequestState)}
}

type LoadOrStoreFN func(fileState *AccessedFileState) (*mcmodel.File, error)

// LoadOrStore calls fn within the context of the mutex lock. It passes the entry
// found at path, or nil if there wasn't an entry at that path. The function can
// then conduct any operations within the context of the lock. If it encounters
// an error then an error is returned from LoadOrStore. If no error is returned from
// the function, but a *mcmodel.File is returned, then a new AccessedFileState entry will
// be created. If there is no error and no *mcmodel.File is returned then nothing
// will happen.
//
// This function exists so that a set of operations can be completed before loading
// a known file. It prevents a race condition where other file system calls made
// don't complete before the known files tracker is loaded.
func (tracker *TransferStateTracker) LoadOrStore(transferRequestKey, path string, fn LoadOrStoreFN) error {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	transferState, ok := tracker.TransferRequestStates[transferRequestKey]
	if !ok {
		transferState = &TransferRequestState{
			AccessedFileStates: make(map[string]*AccessedFileState),
		}
		tracker.TransferRequestStates[transferRequestKey] = transferState
	}

	file := transferState.AccessedFileStates[path]
	potentialNewFile, err := fn(file)
	switch {
	case err != nil:
		return err
	case potentialNewFile == nil:
		return nil
	default:
		// If we are here then a *mcmodel.File was returned. This means
		// we need to add it into the list of known transferState.
		newAccessedFileState := &AccessedFileState{
			File:   potentialNewFile,
			Hasher: md5.New(),
		}
		transferState.AccessedFileStates[path] = newAccessedFileState
		return nil
	}
}

// WithLockHeld finds the AccessedFileState associated with path and calls fn. The call to fn happens
// while the tracker mutex is held. This way fn knows that the AccessedFileState entry cannot change
// while fn is running. WithLockHeld returns true if it found a matching AccessedFileState. It returns
// false if path didn't match a AccessedFileState. When no match is found, fn is **not** called.
func (tracker *TransferStateTracker) WithLockHeld(transferRequestKey, path string, fn func(fileState *AccessedFileState)) bool {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	transferRequestState, ok := tracker.TransferRequestStates[transferRequestKey]
	if !ok {
		// Nothing under first level key
		return false
	}

	fileEntry := transferRequestState.AccessedFileStates[path]
	if fileEntry == nil {
		// File not found
		return false
	}

	// Found a known file
	fn(fileEntry)
	return true
}

// Store grabs the mutex and stores the entry. It returns false if an
// entry already existed (and doesn't update it), otherwise it returns
// true and adds the path and a new AccessedFileState to the tracker.
func (tracker *TransferStateTracker) Store(transferRequestKey, path string, file *mcmodel.File, state string) bool {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	transferRequestState, ok := tracker.TransferRequestStates[transferRequestKey]
	if !ok {
		transferRequestState = &TransferRequestState{
			AccessedFileStates: make(map[string]*AccessedFileState),
		}
		tracker.TransferRequestStates[transferRequestKey] = transferRequestState
	}

	// Now look for the file now that we have the transferRequestState
	if _, ok := transferRequestState.AccessedFileStates[path]; ok {
		// An entry already exists. Don't create a new one
		// and signal this case by returning false.
		return false
	}

	// If we are here then path was not found. Create a new entry and
	// add it to the tracker.
	fileState := &AccessedFileState{
		File:      file,
		Hasher:    md5.New(),
		FileState: state,
	}

	transferRequestState.AccessedFileStates[path] = fileState

	return true
}

// GetFile will return the &mcmodel.File entry in the AccessedFileState if path
// exists. Otherwise, it will return nil.
func (tracker *TransferStateTracker) GetFile(transferRequestKey, path string) *mcmodel.File {
	fileEntry := tracker.Get(transferRequestKey, path)
	if fileEntry != nil {
		return fileEntry.File
	}

	return nil
}

// Get will return the AccessedFileState entry if path exists. Otherwise, it will return nil.
func (tracker *TransferStateTracker) Get(transferRequestKey, path string) *AccessedFileState {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	if _, ok := tracker.TransferRequestStates[transferRequestKey]; !ok {
		return nil
	}

	transferRequestState := tracker.TransferRequestStates[transferRequestKey]
	fileEntry := transferRequestState.AccessedFileStates[path]

	if fileEntry != nil {
		return fileEntry
	}
	return nil
}

// DeletePath will delete path from the tracker. It doesn't check for existence.
func (tracker *TransferStateTracker) DeletePath(transferRequestKey, path string) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	if _, ok := tracker.TransferRequestStates[transferRequestKey]; ok {
		delete(tracker.TransferRequestStates[transferRequestKey].AccessedFileStates, path)
	}
}

func (tracker *TransferStateTracker) DeleteBase(transferRequestKey string) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	delete(tracker.TransferRequestStates, transferRequestKey)
}
