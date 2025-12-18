package stor

import (
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormRemoteClientStor struct {
	db *gorm.DB
}

func NewGormRemoteClientStor(db *gorm.DB) *GormRemoteClientStor {
	return &GormRemoteClientStor{db: db}
}

func (s *GormRemoteClientStor) CreateRemoteClient(RemoteClient *mcmodel.RemoteClient) (*mcmodel.RemoteClient, error) {
	return nil, nil
}

func (s *GormRemoteClientStor) GetRemoteClientByClientID(clientID string) (*mcmodel.RemoteClient, error) {
	return nil, nil
}

func (s *GormRemoteClientStor) AddTransferRequestToRemoteClient(remoteClientID int, transferRequestID int) error {
	return nil
}
