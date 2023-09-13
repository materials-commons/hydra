package stor

import (
	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormGlobusTransferStor struct {
	db *gorm.DB
}

func NewGormGlobusTransferStor(db *gorm.DB) *GormGlobusTransferStor {
	return &GormGlobusTransferStor{db: db}
}

// CreateGlobusTransfer creates a new GlobusTransfer, filling in the ID and UUID for the globus transfer passed in.
func (s *GormGlobusTransferStor) CreateGlobusTransfer(globusTransfer *mcmodel.GlobusTransfer) (*mcmodel.GlobusTransfer, error) {
	var err error

	if globusTransfer.UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	err = WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Create(globusTransfer).Error
	})
	if err != nil {
		return nil, err
	}

	return globusTransfer, nil
}
