package mcfs

import (
	"crypto/md5"
	"hash"
	"sync"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

// The KnownFilesTracker is used to track files the filesystem has already seen. The
// paths that are stored include the user/project combination. This means that there
// could be multiple instances of the same project file path (so not including the
// user/project in the path). This is ok, because each these will represent a
// different file version.
type KnownFilesTracker struct {
	mu         sync.Mutex
	knownFiles map[string]*KnownFile
}

type KnownFile struct {
	file        *mcmodel.File
	hasher      hash.Hash
	hashInvalid bool
	sequence    int
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

func (tracker *KnownFilesTracker) Store(path string, file *mcmodel.File) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	knownFile := &KnownFile{
		file:   file,
		hasher: md5.New(),
	}

	tracker.knownFiles[path] = knownFile
}

func (tracker *KnownFilesTracker) GetFile(path string) *mcmodel.File {
	knownFileEntry := tracker.Get(path)
	if knownFileEntry != nil {
		return knownFileEntry.file
	}

	return nil
}

func (tracker *KnownFilesTracker) Get(path string) *KnownFile {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	knownFileEntry := tracker.knownFiles[path]
	if knownFileEntry != nil {
		return knownFileEntry
	}
	return nil
}

func (tracker *KnownFilesTracker) Delete(path string) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	delete(tracker.knownFiles, path)
}
