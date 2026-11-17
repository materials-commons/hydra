package mcmodel

import (
	"time"
)

type UserApiToken struct {
	ID        int       `json:"id"`
	UUID      string    `json:"uuid"`
	Name      string    `json:"name"`
	Token     string    `json:"-"`
	OwnerID   int       `json:"owner_id"`
	Owner     *User     `json:"owner" gorm:"foreignKey:OwnerID;references:ID"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName returns the name of the table Gorm uses for the file model.
func (UserApiToken) TableName() string {
	return "user_api_tokens"
}
