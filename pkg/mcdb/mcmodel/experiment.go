package mcmodel

import (
	"time"
)

type Experiment struct {
	ID          int       `json:"id"`
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Summary     string    `json:"summary"`
	OwnerID     int       `json:"owner_id"`
	Owner       *User     `json:"owner" gorm:"foreignKey:OwnerID;references:ID"`
	ProjectID   int       `json:"project_id"`
	Project     *Project  `json:"project" gorm:"foreignKey:ProjectID;references:ID"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
