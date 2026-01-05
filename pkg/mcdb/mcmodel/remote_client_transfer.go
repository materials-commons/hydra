package mcmodel

import (
	"time"

	"gorm.io/gorm"
)

type RemoteClientTransfer struct {
	ID               int           `json:"id"`
	UUID             string        `json:"uuid"`
	State            string        `json:"state"`
	TransferID       string        `json:"transfer_id"`
	TransferType     string        `json:"transfer_type"`
	ExpectedSize     uint64        `json:"expected_size"`
	ExpectedChecksum string        `json:"expected_checksum"`
	ChunkSize        int           `json:"chunk_size"`
	RemotePath       string        `json:"remote_path"`
	RemoteClientID   int           `json:"remote_client_id"`
	RemoteClient     *RemoteClient `gorm:"foreignKey:RemoteClientID;references:ID"`
	OwnerID          int           `json:"owner_id"`
	Owner            *User         `gorm:"foreignKey:OwnerID;references:ID"`
	ProjectID        int           `json:"project_id"`
	Project          *Project      `gorm:"foreignKey:ProjectID;references:ID"`
	FileID           int           `json:"file_id"`
	File             *File         `gorm:"foreignKey:FileID;references:ID"`
	LastActiveAt     time.Time     `json:"last_active_at"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

func (r *RemoteClientTransfer) BeforeCreate(tx *gorm.DB) (err error) {
	if r.LastActiveAt.IsZero() {
		r.LastActiveAt = time.Now()
	}
	return
}

func (r *RemoteClientTransfer) BeforeUpdate(tx *gorm.DB) (err error) {
	if r.LastActiveAt.IsZero() {
		r.LastActiveAt = time.Now()
	}
	return
}
