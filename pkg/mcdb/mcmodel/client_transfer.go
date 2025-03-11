package mcmodel

import (
	"time"
)

type ClientTransfer struct {
	ID                int              `json:"id"`
	UUID              string           `json:"uuid"`
	State             string           `json:"state"`
	ClientUUID        string           `json:"client_uuid"`
	ProjectID         int              `json:"project_id"`
	Project           *Project         `json:"project" gorm:"foreignkey:ProjectID;references:ID"`
	OwnerID           int              `json:"owner_id"`
	Owner             *User            `gorm:"foreignKey:OwnerID;references:ID"`
	TransferRequestID int              `json:"transfer_request_id"`
	TransferRequest   *TransferRequest `gorm:"foreignKey:TransferRequestID;references:ID"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

func (ClientTransfer) TableName() string {
	return "client_transfers"
}
