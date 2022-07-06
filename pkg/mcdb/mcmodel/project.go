package mcmodel

import (
	"encoding/json"
	"time"
)

type Project struct {
	ID             int       `json:"id"`
	UUID           string    `json:"uuid"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	TeamID         int       `json:"team_id"`
	OwnerID        int       `json:"owner_id"`
	Owner          *User     `json:"owner" gorm:"foreignKey:OwnerID;references:ID"`
	Size           int64     `json:"size"`
	FileCount      int       `json:"file_count"`
	DirectoryCount int       `json:"directory_count"`
	FileTypes      string    `json:"file_types"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (p Project) GetFileTypes() (map[string]int, error) {
	var fileTypes map[string]int
	err := json.Unmarshal([]byte(p.FileTypes), &fileTypes)
	return fileTypes, err
}

func (p Project) ToFileTypeAsString(fileTypes map[string]int) (string, error) {
	b, err := json.Marshal(fileTypes)
	return string(b), err
}
