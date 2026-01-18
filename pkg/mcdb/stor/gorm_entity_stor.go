package stor

import (
	"fmt"

	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormEntityStor struct {
	db *gorm.DB
}

func NewGormEntityStor(db *gorm.DB) *GormEntityStor {
	return &GormEntityStor{db: db}
}

func (s *GormEntityStor) GetProjectEntityByID(projectID int, entityID int) (*mcmodel.Entity, error) {
	var entity mcmodel.Entity
	err := s.db.Find(&entity, entityID).Error
	if err != nil {
		return nil, err
	}

	if entity.ProjectID != projectID {
		return nil, fmt.Errorf("entity %d not found for project %d", entityID, projectID)
	}

	return &entity, nil
}

func (s *GormEntityStor) ListProjectEntitiesByCategory(projectID int, category string) ([]mcmodel.Entity, error) {
	var entities []mcmodel.Entity
	err := s.db.Where("project_id = ? AND category = ?", projectID, category).Find(&entities).Error
	return nil, err
}

func (s *GormEntityStor) CreateEntity(entity *mcmodel.Entity) (*mcmodel.Entity, error) {
	var err error

	if entity.UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	err = WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Create(entity).Error
	})

	return entity, err
}
