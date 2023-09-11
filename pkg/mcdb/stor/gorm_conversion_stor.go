package stor

import (
	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormConversionStor struct {
	db *gorm.DB
}

func NewGormConversionStor(db *gorm.DB) *GormConversionStor {
	return &GormConversionStor{db: db}
}

func (s *GormConversionStor) AddFileToConvert(file *mcmodel.File) (*mcmodel.Conversion, error) {
	var err error

	c := &mcmodel.Conversion{
		ProjectID: file.ProjectID,
		FileID:    file.ID,
		OwnerID:   file.OwnerID,
	}

	if c.UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	err = WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Create(c).Error
	})

	return c, err
}
