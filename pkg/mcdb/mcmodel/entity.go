package mcmodel

import (
	"time"
)

type Entity struct {
	ID           int           `json:"id"`
	UUID         string        `json:"uuid"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Summary      string        `json:"summary"`
	Category     string        `json:"category"`
	ProjectID    int           `json:"project_id"`
	Project      *Project      `json:"project" gorm:"foreignKey:ProjectID;references:ID"`
	OwnerID      int           `json:"owner_id"`
	Owner        *User         `json:"owner" gorm:"foreignKey:OwnerID;references:ID"`
	DatasetID    *int          `json:"dataset_id"`
	CopiedID     *int          `json:"copied_id"`
	Files        []File        `json:"files" gorm:"many2many:entity2file"`
	EntityStates []EntityState `json:"entity_states"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

type EntityState struct {
	ID         int         `json:"id"`
	UUID       string      `json:"uuid"`
	EntityID   int         `json:"entity_id"`
	Current    bool        `json:"current"`
	OwnerID    int         `json:"owner_id"`
	DatasetID  int         `json:"dataset_id"`
	Attributes []Attribute `json:"attributes" gorm:"polymorphic:Attributable;polymorphicValue:App\\Models\\EntityState"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}
