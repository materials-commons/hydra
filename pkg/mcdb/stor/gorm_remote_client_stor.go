package stor

import (
	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormRemoteClientStor struct {
	db *gorm.DB
}

func NewGormRemoteClientStor(db *gorm.DB) *GormRemoteClientStor {
	return &GormRemoteClientStor{db: db}
}

func (s *GormRemoteClientStor) CreateRemoteClient(remoteClient *mcmodel.RemoteClient) (*mcmodel.RemoteClient, error) {
	var (
		err error
	)

	if remoteClient.UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	err = WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Create(remoteClient).Error
	})

	if err != nil {
		return nil, err
	}
	return remoteClient, nil
}

func (s *GormRemoteClientStor) GetRemoteClientByClientID(clientID string) (*mcmodel.RemoteClient, error) {
	var remoteClient mcmodel.RemoteClient
	err := s.db.Where("client_id = ?", clientID).First(&remoteClient).Error
	if err != nil {
		return nil, err
	}

	return &remoteClient, nil
}
