package stor

import (
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormClientTransferStor struct {
	db *gorm.DB
}

func NewGormClientTransferStor(db *gorm.DB) *GormClientTransferStor {
	return &GormClientTransferStor{db: db}
}

func (s *GormClientTransferStor) CreateClientTransfer(ct *mcmodel.ClientTransfer) (*mcmodel.ClientTransfer, error) {
	return nil, nil
}

func (s *GormClientTransferStor) GetOrCreateClientTransferByPath(clientUUID string, projectID, ownerID int, filePath string) (*mcmodel.ClientTransfer, error) {
	return nil, nil
}

func (s *GormClientTransferStor) UpdateClientTransfer(ct *mcmodel.ClientTransfer) (*mcmodel.ClientTransfer, error) {
	return nil, nil
}

func (s *GormClientTransferStor) CloseClientTransfer(clientTransferID int) error {
	return nil
}

func (s *GormClientTransferStor) AbortClientTransfer(clientTransferID int) error {
	return nil
}
