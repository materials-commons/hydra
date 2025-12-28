package stor

import (
	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormRemoteClientTransferStor struct {
	db *gorm.DB
}

func NewGormRemoteClientTransferStor(db *gorm.DB) *GormRemoteClientTransferStor {
	return &GormRemoteClientTransferStor{db: db}
}

func (s *GormRemoteClientTransferStor) CreateRemoteClientTransfer(clientTransfer *mcmodel.RemoteClientTransfer) (*mcmodel.RemoteClientTransfer, error) {
	var (
		err error
	)

	if clientTransfer.UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	err = WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Create(clientTransfer).Error
	})

	if err != nil {
		return nil, err
	}
	return clientTransfer, nil
}

func (s *GormRemoteClientTransferStor) GetRemoteClientTransferByTransferID(transferID string) (*mcmodel.RemoteClientTransfer, error) {
	var transfer mcmodel.RemoteClientTransfer
	err := s.db.Preload("File.Directory").
		Where("transfer_id = ?", transferID).
		First(&transfer).Error
	if err != nil {
		return nil, err
	}
	return &transfer, nil
}

func (s *GormRemoteClientTransferStor) UpdateRemoteClientTransferState(transferID string, state string) (*mcmodel.RemoteClientTransfer, error) {
	err := WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Model(&mcmodel.RemoteClientTransfer{}).Where("transfer_id = ?", transferID).Update("state", state).Error
	})

	if err != nil {
		return nil, err
	}

	return s.GetRemoteClientTransferByTransferID(transferID)
}

func (s *GormRemoteClientTransferStor) GetAllTransfersForRemoteClient(remoteClientID int) ([]mcmodel.RemoteClientTransfer, error) {
	var transfers []mcmodel.RemoteClientTransfer
	err := s.db.Where("remote_client_id = ?", remoteClientID).Find(&transfers).Error
	return nil, err
}

func (s *GormRemoteClientTransferStor) GetAllUploadTransfersForRemoteClient(remoteClientID int) ([]mcmodel.RemoteClientTransfer, error) {
	var transfers []mcmodel.RemoteClientTransfer
	err := s.db.Where("remote_client_id = ? and transfer_type = ?", remoteClientID, "upload").Find(&transfers).Error
	return transfers, err
}

func (s *GormRemoteClientTransferStor) GetAllDownloadTransfersForRemoteClient(remoteClientID int) ([]mcmodel.RemoteClientTransfer, error) {
	var transfers []mcmodel.RemoteClientTransfer
	err := s.db.Where("remote_client_id = ? and transfer_type = ?", remoteClientID, "download").Find(&transfers).Error
	return transfers, err
}
