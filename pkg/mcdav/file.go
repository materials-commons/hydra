package mcdav

import (
	"context"
	"os"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type MCFile struct {
	*os.File
	fileStor stor.FileStor
	mcfile   *mcmodel.File
}

func NewMCFile(osf *os.File, fileStor stor.FileStor, mcfile *mcmodel.File) *MCFile {
	return &MCFile{
		File:     osf,
		fileStor: fileStor,
		mcfile:   mcfile,
	}
}

func (f *MCFile) Close() error {
	err := f.File.Close()
	// Update metadata in materials commons
	return err
}

func (f *MCFile) Write(p []byte) (n int, err error) {
	// write to hash
	return f.File.Write(p)
}

func (f *MCFile) ContentType(ctx context.Context) (string, error) {
	return f.mcfile.MimeType, nil
}
