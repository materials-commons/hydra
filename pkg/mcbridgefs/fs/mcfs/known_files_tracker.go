package mcfs

import (
	"crypto/md5"
	"hash"
	"sync"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

// The KnownFilesTracker is used to track files the filesystem has already seen. The
// paths that are stored include the user/project combination. This means that there
// could be multiple instances of the same project file path, but different for
// each user. This is ok, because each these will represent a different file version
// because each user will get their own version when opened for write.
//
// Note: Files are only stored if opened for write. That means if a file is opened
// for write, then later opened for read, it will refer to the version opened for write.
// All calls on a KnownFilesTracker are thread safe.
type KnownFilesTracker struct {
	// Synchronizes access to the knownFiles map
	mu sync.Mutex

	// A map of known files, where the key is the path including the project and user id.
	// For example, if user id 1 is opening file /dir1/file.txt in project id 2, then
	// the key will be /2/1/dir1/file.txt. The constructed path is project-id/user-id/...
	knownFiles map[string]*KnownFile
}

// KnownFile tracks the state of a file that was opened for write. This tracks the underlying
// database file entry and the hasher used to construct the checksum.
type KnownFile struct {
	// file is the database file the file system file is associated with
	file *mcmodel.File

	// hasher is used to create the checksum. As writes are done to a file the
	// checksum is updated.
	hasher hash.Hash

	// hashInvalid is set to true when a user seeks or truncates a file. When
	// that happens the hash state is invalid.
	hashInvalid bool

	// sequence is used when hashInvalid is set to true. The sequence is used by
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
	// matching the sequence in the KnownFile entry will update the checksum.
	sequence int
}

func NewKnownFilesTracker() *KnownFilesTracker {
	return &KnownFilesTracker{knownFiles: make(map[string]*KnownFile)}
}

type LoadOrStoreFN func(knownFile *KnownFile) (*mcmodel.File, error)

// LoadOrStore calls fn within the context of the mutex lock. It passes the entry
// found at path, or nil if there wasn't an entry at that path. The function can
// then conduct any operations within the context of the lock. If it encounters
// an error then an error is returned from LoadOrStore. If no error is returned from
// the function, but a *mcmodel.File is returned, then a new KnownFile entry will
// be created. If there is no error and no *mcmodel.File is returned then nothing
// will happen.
//
// This function exists so that a set of operations can be completed before loading
// a known file. It prevents a race condition where other file system calls made
// don't complete before the known files tracker is loaded.
func (tracker *KnownFilesTracker) LoadOrStore(path string, fn LoadOrStoreFN) error {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	knownFile := tracker.knownFiles[path]
	potentialNewFile, err := fn(knownFile)
	switch {
	case err != nil:
		return err
	case potentialNewFile == nil:
		return nil
	default:
		// If we are here then a *mcmodel.File was returned. This means
		// we need to add it into the list of known files.
		newKnownFile := &KnownFile{
			file:   potentialNewFile,
			hasher: md5.New(),
		}
		tracker.knownFiles[path] = newKnownFile
		return nil
	}
}

// WithLockHeld finds the KnownFile associated with path and calls fn. The call to fn happens
// while the tracker mutex is held. This way fn knows that the KnownFile entry cannot change
// while fn is running. WithLockHeld returns true if it found a matching KnownFile. It returns
// false if path didn't match a KnownFile. When no match is found, fn is **not** called.
func (tracker *KnownFilesTracker) WithLockHeld(path string, fn func(knownFile *KnownFile)) bool {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	knownFileEntry := tracker.knownFiles[path]
	if knownFileEntry == nil {
		// File not found
		return false
	}

	// Found a known file
	fn(knownFileEntry)
	return true
}

// Store grabs the mutex and stores the entry. It returns false if an
// entry already existed (and doesn't update it), otherwise it returns
// true and adds the path and a new KnownFile to the tracker.
func (tracker *KnownFilesTracker) Store(path string, file *mcmodel.File) bool {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	if _, ok := tracker.knownFiles[path]; ok {
		// An entry already exists. Don't create a new one
		// and signal this case by returning false.
		return false
	}

	// If we are here then path was not found. Create a new entry and
	// add it to the tracker.
	knownFile := &KnownFile{
		file:   file,
		hasher: md5.New(),
	}

	tracker.knownFiles[path] = knownFile

	return true
}

// GetFile will return the &mcmodel.File entry in the KnownFile if path
// exists. Otherwise, it will return nil.
func (tracker *KnownFilesTracker) GetFile(path string) *mcmodel.File {
	knownFileEntry := tracker.Get(path)
	if knownFileEntry != nil {
		return knownFileEntry.file
	}

	return nil
}

// Get will return the KnownFile entry if path exists. Otherwise, it will return nil.
func (tracker *KnownFilesTracker) Get(path string) *KnownFile {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	knownFileEntry := tracker.knownFiles[path]
	if knownFileEntry != nil {
		return knownFileEntry
	}
	return nil
}

// Delete will delete path from the tracker. It doesn't check for existence.
func (tracker *KnownFilesTracker) Delete(path string) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	delete(tracker.knownFiles, path)
}
