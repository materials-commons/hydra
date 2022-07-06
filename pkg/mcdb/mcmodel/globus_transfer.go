package mcmodel

import "time"

type GlobusTransfer struct {
	ID                                int              `json:"id"`
	UUID                              string           `json:"string"`
	ProjectID                         int              `json:"project_id"`
	Name                              string           `json:"name"`
	State                             string           `json:"state"`
	OwnerID                           int              `json:"owner_id"`
	Owner                             *User            `gorm:"foreignKey:OwnerID;references:ID"`
	GlobusEndpointID                  string           `json:"globus_endpoint_id"`
	GlobusAclID                       string           `json:"globus_acl_id"`
	GlobusPath                        string           `json:"globus_path"`
	GlobusIdentityID                  string           `json:"globus_identity_id"`
	GlobusURL                         string           `json:"globus_url"`
	LastGlobusTransferIDCompleted     string           `gorm:"column:last_globus_transfer_id_completed" json:"last_globus_transfer_id_completed"`
	LatestGlobusTransferCompletedDate string           `json:"latest_globus_transfer_completed_date"`
	TransferRequestID                 int              `json:"transfer_request_id"`
	TransferRequest                   *TransferRequest `gorm:"foreignKey:TransferRequestID;references:ID"`
	CreatedAt                         time.Time        `json:"created_at"`
	UpdatedAt                         time.Time        `json:"updated_at"`
}

func (GlobusTransfer) TableName() string {
	return "globus_transfers"
}
