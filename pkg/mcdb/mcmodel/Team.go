package mcmodel

import (
	"time"
)

type Team struct {
	ID        int       `json:"id"`
	UUID      string    `json:"uuid"`
	Name      string    `json:"name"`
	OwnerID   int       `json:"owner_id"`
	Owner     *User     `json:"owner" gorm:"foreignKey:OwnerID;references:ID"`
	Members   []User    `gorm:"many2many:team2member;"`
	Admins    []User    `gorm:"many2many:team2admin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
