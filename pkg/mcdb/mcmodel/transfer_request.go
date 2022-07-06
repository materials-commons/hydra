package mcmodel

import "time"

type TransferRequest struct {
	ID             int             `json:"id"`
	UUID           string          `json:"uuid"`
	State          string          `json:"state"`
	ProjectID      int             `json:"project_id"`
	OwnerID        int             `json:"owner_id"`
	Owner          *User           `json:"owner" gorm:"foreignKey:OwnerID;references:ID"`
	GlobusTransfer *GlobusTransfer `json:"globus_transfer" gorm:"foreignKey:transfer_request_id;references:id"`
	LastActiveAt   time.Time       `json:"last_active_at"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func (TransferRequest) TableName() string {
	return "transfer_requests"
}
