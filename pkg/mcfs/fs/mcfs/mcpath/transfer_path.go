package mcpath

import (
	"fmt"
	"path/filepath"

	"github.com/apex/log"
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

func (p *TransferPath) TransferKey() string {
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
	switch p.pathType {
	case BadPathType:
		return nil, fmt.Errorf("bad path: %s\n", p.fullPath)
	case RootPathType:
		return nil, fmt.Errorf("root not supported")
	case ContextPathType:
		f := &mcmodel.File{
			Name:      p.transferRequest.UUID,
			MimeType:  "directory",
			Path:      filepath.Join("/", p.transferRequest.UUID),
			Directory: &mcmodel.File{Path: "/", Name: "/", MimeType: "directory"},
		}
		return f, nil
	default:
		f, err := p.stors.FileStor.GetFileByPath(p.ProjectID(), p.projectPath)
		return f, err
	}
}

func (p *TransferPath) List() ([]mcmodel.File, error) {
	log.Debugf("TransferPath.List %#v", p)
	switch p.pathType {
	case BadPathType:
		return nil, fmt.Errorf("pathType for TransferPath type is BadPath")
	case RootPathType:
		return p.listTransferRequests()
	default:
		return p.listProjectDirectory()
	}
}

func (p *TransferPath) listTransferRequests() ([]mcmodel.File, error) {
	transferRequests, err := p.stors.TransferRequestStor.ListTransferRequests()
	if err != nil {
		return nil, err
	}
	inDir := &mcmodel.File{Path: "/", MimeType: "directory"}
	var dirEntries []mcmodel.File
	for _, tr := range transferRequests {
		entry := mcmodel.File{
			Directory: inDir,
			Name:      fmt.Sprintf("%s", tr.UUID),
			Path:      fmt.Sprintf("/%s", tr.UUID),
			MimeType:  "directory",
		}
		dirEntries = append(dirEntries, entry)
	}
	return dirEntries, nil
}

func (p *TransferPath) listProjectDirectory() ([]mcmodel.File, error) {
	log.Debugf("TransferPath.listProjectDirectory %d, path = %s", p.ProjectID(), p.projectPath)
	dir, err := p.stors.FileStor.GetDirByPath(p.ProjectID(), p.projectPath)
	if err != nil {
		return nil, err
	}

	log.Debugf("GetDirByPath returned = %#v", dir)
	// Make list directory to a pointer for transferRequest?
	dirEntries, err := p.stors.TransferRequestStor.ListDirectory(dir, p.transferRequest)
	if err != nil {
		log.Debugf("ListDirectory returned error %s", err)
		return nil, err
	}

	for i := 0; i < len(dirEntries); i++ {
		dirEntries[i].Directory = dir
	}

	return dirEntries, nil
}
