package mcpath

import (
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type TransferPath struct {
	pathType        PathType
	projectPath     string
	fullPath        string
	transferBase    string
	transferRequest *mcmodel.TransferRequest
}

func (p *TransferPath) ProjectID() int {
	if p.transferRequest == nil {
		return -1
	}
	return p.transferRequest.ProjectID
}

func (p *TransferPath) UserID() int {
	if p.transferRequest == nil {
		return -1
	}
	return p.transferRequest.OwnerID
}

func (p *TransferPath) TransferID() int {
	if p.transferRequest == nil {
		return -1
	}
	return p.transferRequest.ID
}

func (p *TransferPath) TransferUUID() string {
	if p.transferRequest == nil {
		return ""
	}
	return p.transferRequest.UUID
}

func (p *TransferPath) ProjectPath() string {
	return p.projectPath
}

func (p *TransferPath) FullPath() string {
	return p.fullPath
}

func (p *TransferPath) TransferBase() string {
	return p.transferBase
}

func (p *TransferPath) PathType() PathType {
	return p.pathType
}
