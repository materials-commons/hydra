package mcpath

import (
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type TransferPath struct {
	projectPath     string
	fullPath        string
	transferBase    string
	transferRequest *mcmodel.TransferRequest
}

func (p *TransferPath) ProjectID() int {
	return p.transferRequest.ProjectID
}

func (p *TransferPath) UserID() int {
	return p.transferRequest.OwnerID
}

func (p *TransferPath) TransferID() int {
	return p.transferRequest.ID
}

func (p *TransferPath) TransferUUID() string {
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
