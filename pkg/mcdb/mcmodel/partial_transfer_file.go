package mcmodel

import (
	"time"
)

type PartialTransferFile struct {
	ID           int
	UUID         string
	TransferID   string
	UserID       int
	ProjectID    int
	DirectoryID  int
	FileName     string
	FilePath     string // Where partial file is being written
	ExpectedSize int64
	ChunkSize    int
	Status       string // "uploading", "complete", "failed"
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (PartialTransferFile) TableName() string {
	return "partial_transfer_files"
}
