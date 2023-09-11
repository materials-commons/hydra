package stor

import (
	"fmt"

	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type InMemoryTransferRequestStor struct {
	ErrToReturn        error
	transferRequests   []mcmodel.TransferRequest
	ListDirectoryFiles []mcmodel.File
	lastID             int
}

func NewInMemoryTransferRequestStor(transferRequests []mcmodel.TransferRequest) *InMemoryTransferRequestStor {
	return &InMemoryTransferRequestStor{
		transferRequests: transferRequests,
		lastID:           10000,
	}
}

func (s *InMemoryTransferRequestStor) MarkFileReleased(file *mcmodel.File, checksum string, projectID int, totalBytes int64) error {
	return ErrNotImplemented
}

func (s *InMemoryTransferRequestStor) MarkFileAsOpen(file *mcmodel.File) error {
	return ErrNotImplemented
}

func (s *InMemoryTransferRequestStor) CreateNewFile(file, dir *mcmodel.File, transferRequest *mcmodel.TransferRequest) (*mcmodel.File, error) {
	if s.ErrToReturn != nil {
		return nil, s.ErrToReturn
	}

	var (
		err   error
		fuuid string
	)

	if fuuid, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	id := s.lastID
	s.lastID = s.lastID + 1
	return &mcmodel.File{
		ID:          id,
		UUID:        fuuid,
		Name:        file.Name,
		DirectoryID: dir.ID,
		OwnerID:     file.OwnerID,
		ProjectID:   file.ProjectID,
	}, nil
}

func (s *InMemoryTransferRequestStor) CreateNewFileVersion(file, dir *mcmodel.File, transferRequest *mcmodel.TransferRequest) (*mcmodel.File, error) {
	return nil, ErrNotImplemented
}

func (s *InMemoryTransferRequestStor) ListDirectory(dir *mcmodel.File, transferRequest *mcmodel.TransferRequest) ([]mcmodel.File, error) {
	if s.ErrToReturn != nil {
		return nil, s.ErrToReturn
	}

	return s.ListDirectoryFiles, nil
}

func (s *InMemoryTransferRequestStor) GetFileByPath(path string, transferRequest *mcmodel.TransferRequest) (*mcmodel.File, error) {
	return nil, ErrNotImplemented
}

func (s *InMemoryTransferRequestStor) GetTransferRequestByProjectAndUser(projectID, userID int) (*mcmodel.TransferRequest, error) {
	if s.ErrToReturn != nil {
		return nil, s.ErrToReturn
	}

	for _, tr := range s.transferRequests {
		if tr.ProjectID == projectID && tr.OwnerID == userID {
			return &tr, nil
		}
	}

	return nil, fmt.Errorf("no such transfer request")
}
