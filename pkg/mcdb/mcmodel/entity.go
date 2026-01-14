package mcmodel

import (
	"time"
)

type Entity struct {
	ID           int           `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Summary      string        `json:"summary"`
	Category     string        `json:"category"`
	ProjectID    int           `json:"project_id"`
	OwnerID      int           `json:"owner_id"`
	Files        []File        `json:"files" gorm:"many2many:entity2file"`
	EntityStates []EntityState `json:"entity_states"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

type EntityState struct {
	ID         int         `json:"id"`
	EntityID   int         `json:"entity_id"`
	Attributes []Attribute `json:"attributes" gorm:"-"`
}
