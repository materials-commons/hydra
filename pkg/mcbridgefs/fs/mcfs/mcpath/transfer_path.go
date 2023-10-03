package mcpath

import (
	"fmt"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type TransferPath struct {
	pathType        PathType
	projectPath     string
	fullPath        string
	transferBase    string
	transferRequest *mcmodel.TransferRequest
	stors           *stor.Stors
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

func (p *TransferPath) Lookup() (*mcmodel.File, error) {
	return nil, nil
}

func (p *TransferPath) List() ([]mcmodel.File, error) {
	switch p.pathType {
	case RootPathType:
		return p.listTransferRequests()
	case ContextPathType:
		return p.listProjectRoot()
	case ProjectPathType:
		return p.listProjectDir()
	case BadPathType:
		return nil, fmt.Errorf("pathType for TransferPath type is BadPath")
	default:
		return nil, fmt.Errorf("pathType for Transfer is unknown")
	}
}

func (p *TransferPath) listTransferRequests() ([]mcmodel.File, error) {
	return nil, nil
}

func (p *TransferPath) listProjectRoot() ([]mcmodel.File, error) {
	return nil, nil
}

func (p *TransferPath) listProjectDir() ([]mcmodel.File, error) {
	return nil, nil
}
