package mcmodel

import "time"

type Conversion struct {
	ID                  int       `json:"id"`
	UUID                string    `json:"uuid"`
	ProjectID           int       `json:"project_id"`
	OwnerID             int       `json:"owner_id"`
	FileID              int       `json:"file_id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	ConversionStartedAt time.Time `json:"conversion_started_at"`
}

func (Conversion) TableName() string {
	return "conversions"
}
