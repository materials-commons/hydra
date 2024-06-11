package handler

import (
	"context"
	"io"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcssh/mc"
	"github.com/tus/tusd/v2/pkg/handler"
)

type MCFileStore struct {
	fileStor    stor.FileStor
	ProjectID   int
	DirectoryID int
	OwnerID     int
}

func NewMCFileStore() *MCFileStore {
	return &MCFileStore{}
}

// NewUpload creates a new upload using the size as the file's length. The method must
// return a unique id which is used to identify the upload. If no backend
// (e.g. Riak) specifes the id you may want to use the uid package to
// generate one. The properties Size and MetaData will be filled.
func (s *MCFileStore) NewUpload(ctx context.Context, info handler.FileInfo) (upload handler.Upload, err error) {
	mcfile, err := s.fileStor.CreateFile("", s.ProjectID, s.DirectoryID, s.OwnerID, mc.GetMimeType(""))
	if err != nil {
		return nil, err
	}

	_ = mcfile
	return nil, nil
}

// GetUpload fetches the upload with a given ID. If no such upload can be found,
// ErrNotFound must be returned.
func (s *MCFileStore) GetUpload(ctx context.Context, id string) (upload handler.Upload, err error) {
	return nil, nil
}

func (s *MCFileStore) UseIn(composer *handler.StoreComposer) {
	composer.UseCore(s)
	composer.UseTerminater(s)
	composer.UseConcater(s)
	composer.UseLengthDeferrer(s)
}

func (s *MCFileStore) AsTerminatableUpload(upload handler.Upload) handler.TerminatableUpload {
	return nil
}

func (s *MCFileStore) AsLengthDeclarableUpload(upload handler.Upload) handler.LengthDeclarableUpload {
	return nil
}

func (s *MCFileStore) AsConcatableUpload(upload handler.Upload) handler.ConcatableUpload {
	return nil
}

type MCFileUpload struct {
	MCFile *mcmodel.File
}

func (u *MCFileUpload) GetInfo(ctx context.Context) (info handler.FileInfo, err error) {
	return handler.FileInfo{}, nil
}

func (u *MCFileUpload) WriteChunk(ctx context.Context, offset int64, src io.Reader) (int64, error) {
	return 0, nil
}

func (u *MCFileUpload) GetReader(ctx context.Context) (io.ReadCloser, error) {
	return nil, nil
}

func (u *MCFileUpload) Terminate(ctx context.Context) error {
	return nil
}

func (u *MCFileUpload) ConcatUploads(ctx context.Context, uploads []handler.Upload) (err error) {
	return nil
}

func (u *MCFileUpload) DeclareLength(ctx context.Context, length int64) error {
	return nil
}

func (u *MCFileUpload) FinishUpload(ctx context.Context) error {
	return nil
}
