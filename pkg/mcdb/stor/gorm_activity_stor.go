package stor

import (
	"fmt"

	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormActivityStor struct {
	db *gorm.DB
}

func NewGormActivityStor(db *gorm.DB) *GormActivityStor {
	return &GormActivityStor{db: db}
}

func (s *GormActivityStor) GetProjectActivityByID(projectID int, activityID int) (*mcmodel.Activity, error) {
	var activity mcmodel.Activity

	err := s.db.Find(&activity, activityID).Error
	if err != nil {
		return nil, err
	}

	if activity.ProjectID != projectID {
		return nil, fmt.Errorf("activity %d not found for project %d", activityID, projectID)
	}

	return &activity, err
}

func (s *GormActivityStor) CreateActivity(activity *mcmodel.Activity) (*mcmodel.Activity, error) {
	var err error
	if activity.UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	err = WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Create(activity).Error
	})

	if err != nil {
		return nil, err
	}

	return activity, nil
}
