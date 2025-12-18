package mcmodel

import (
	"time"
)

type RemoteClient struct {
	ID               int               `json:"id"`
	UUID             string            `json:"uuid"`
	State            string            `json:"state"`
	ClientID         string            `json:"client_id"`
	Hostname         string            `json:"hostname"`
	Name             string            `json:"name"`
	TransferRequests []TransferRequest `json:"transfer_requests" gorm:"many2many:remote_client2transfer_request"`
	OwnerID          int               `json:"owner_id"`
	Owner            *User             `gorm:"foreignKey:OwnerID;references:ID"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}
