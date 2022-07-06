package mcmodel

import "time"

type TransferRequestFile struct {
	ID                int              `json:"id"`
	UUID              string           `json:"string"`
	Name              string           `json:"name"`
	State             string           `json:"state"`
	TransferRequestID int              `json:"transfer_request_id"`
	TransferRequest   *TransferRequest `gorm:"foreignKey:TransferRequestID;references:ID"`
	ProjectID         int              `json:"project_id"`
	DirectoryID       int              `json:"directory_id"`
	FileID            int              `json:"file_id"`
	File              *File            `gorm:"foreignKey:FileID;references:ID"`
	OwnerID           int              `json:"owner_id"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

func (TransferRequestFile) TableName() string {
	return "transfer_request_files"
}
