package store

import (
	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/gomcdb/mcmodel"
	"gorm.io/gorm"
)

type GormConversionStore struct {
	db *gorm.DB
}

func NewGormConversionStore(db *gorm.DB) *GormConversionStore {
	return &GormConversionStore{db: db}
}

func (s *GormConversionStore) AddFileToConvert(file *mcmodel.File) (*mcmodel.Conversion, error) {
	var err error

	c := &mcmodel.Conversion{
		ProjectID: file.ProjectID,
		FileID:    file.ID,
		OwnerID:   file.OwnerID,
	}

	if c.UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	err = s.withTxRetry(func(tx *gorm.DB) error {
		return tx.Create(c).Error
	})

	return c, err
}

func (s *GormConversionStore) withTxRetry(fn func(tx *gorm.DB) error) error {
	return WithTxRetryDefault(fn, s.db)
}
