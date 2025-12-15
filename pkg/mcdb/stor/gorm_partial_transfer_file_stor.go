package stor

import (
	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormPartialTransferFileStor struct {
	db *gorm.DB
}

func NewGormPartialTransferFileStor(db *gorm.DB) *GormPartialTransferFileStor {
	return &GormPartialTransferFileStor{db: db}
}

func (s *GormPartialTransferFileStor) CreatePartialTransferFile(ptf *mcmodel.PartialTransferFile) (*mcmodel.PartialTransferFile, error) {
	var err error

	if ptf.UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	err = WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Create(ptf).Error
	})

	if err != nil {
		return nil, err
	}

	return ptf, nil
}

func (s *GormPartialTransferFileStor) UpdateFileSize(transferID string, size int64) error {
	return nil
}

func (s *GormPartialTransferFileStor) GetUploadingFiles() ([]mcmodel.PartialTransferFile, error) {
	return nil, nil
}

func (s *GormPartialTransferFileStor) GetPartialTransferFileByID(transferID string) (*mcmodel.PartialTransferFile, error) {
	return nil, nil
}

func (s *GormPartialTransferFileStor) DeletePartialTransferFile(transferID string) error {
	return nil
}

func (s *GormPartialTransferFileStor) MarkComplete(transferID string, hash string) error {
	return nil
}

func (s *GormPartialTransferFileStor) MarkFailed(transferID string, reason string) error {
	return nil
}
