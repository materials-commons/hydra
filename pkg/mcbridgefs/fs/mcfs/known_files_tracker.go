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
	m sync.Map
}

type KnownFile struct {
	file                 *mcmodel.File
	hasher               hash.Hash
	hashNeedsRecomputing bool
}

func NewKnownFilesTracker() *KnownFilesTracker {
	return &KnownFilesTracker{}
}

func (t *KnownFilesTracker) Store(path string, file *mcmodel.File) {
	knownFile := &KnownFile{
		file:   file,
		hasher: md5.New(),
	}
	t.m.Store(path, knownFile)
}

func (t *KnownFilesTracker) GetFile(path string) *mcmodel.File {
	val, _ := t.m.Load(path)
	if val != nil {
		return val.(*KnownFile).file
	}

	return nil
}

func (t *KnownFilesTracker) Get(path string) *KnownFile {
	val, _ := t.m.Load(path)
	if val != nil {
		return val.(*KnownFile)
	}

	return nil
}

func (t *KnownFilesTracker) Delete(path string) {
	t.m.Delete(path)
}
