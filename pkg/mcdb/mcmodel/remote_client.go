package mcmodel

import (
	"time"

	"gorm.io/gorm"
)

type RemoteClient struct {
	ID                    int                    `json:"id"`
	UUID                  string                 `json:"uuid"`
	State                 string                 `json:"state"`
	ClientID              string                 `json:"client_id"`
	Hostname              string                 `json:"hostname"`
	Name                  string                 `json:"name"`
	Version               string                 `json:"version"`
	Type                  string                 `json:"type"`
	RemoteClientTransfers []RemoteClientTransfer `gorm:"foreignKey:RemoteClientID"`
	OwnerID               int                    `json:"owner_id"`
	Owner                 *User                  `gorm:"foreignKey:OwnerID;references:ID"`
	LastSeenAt            time.Time              `json:"last_seen_at"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
}

func (RemoteClient) TableName() string {
	return "remote_clients"
}

func (r *RemoteClient) BeforeCreate(tx *gorm.DB) (err error) {
	if r.LastSeenAt.IsZero() {
		r.LastSeenAt = time.Now()
	}
	return
}

func (r *RemoteClient) BeforeUpdate(tx *gorm.DB) (err error) {
	if r.LastSeenAt.IsZero() {
		r.LastSeenAt = time.Now()
	}
	return
}
