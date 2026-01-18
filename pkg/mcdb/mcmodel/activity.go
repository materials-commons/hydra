package mcmodel

import (
	"time"
)

type Activity struct {
	ID          int         `json:"id"`
	UUID        string      `json:"uuid"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Summary     string      `json:"summary"`
	Category    string      `json:"category"`
	ProjectID   int         `json:"project_id"`
	Project     *Project    `json:"project" gorm:"foreignKey:ProjectID;references:ID"`
	OwnerID     int         `json:"owner_id"`
	Owner       *User       `json:"owner" gorm:"foreignKey:OwnerID;references:ID"`
	Attributes  []Attribute `json:"attributes" gorm:"polymorphic:Attributable;polymorphicValue:App\\Models\\Activity"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
