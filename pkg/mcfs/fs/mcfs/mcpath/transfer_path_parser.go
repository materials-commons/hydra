package mcpath

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type TransferPathParser struct {
	mu               sync.Mutex
	transferRequests map[string]*mcmodel.TransferRequest
	stors            *stor.Stors
}

func NewTransferPathParser(stors *stor.Stors) ParserReleaser {
	return &TransferPathParser{
		transferRequests: make(map[string]*mcmodel.TransferRequest),
		stors:            stors,
	}
}

func (p *TransferPathParser) Parse(path string) (Path, error) {
	path = filepath.Clean(path)
	if path == "/" {
		return &TransferPath{pathType: RootPathType, fullPath: path, stors: p.stors}, nil
	}

	pathParts := strings.Split(path, "/")
	// A fully formed path looks as follows:
	// pathParts[0] = ""
	// pathParts[1] = transfer-uuid
	// pathParts[2:] path to use for project path

	switch len(pathParts) {
	case 2:
		return p.handleTransferUUIDPath(path, pathParts[1])
	default:
		return p.handleProjectPath(path, pathParts)
	}
}

func (p *TransferPathParser) handleTransferUUIDPath(path string, transferUUID string) (Path, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if tr := p.transferRequests[transferUUID]; tr != nil {
		transferPath := &TransferPath{
			pathType:        ContextPathType,
			fullPath:        path,
			transferBase:    path,
			transferRequest: tr,
			stors:           p.stors,
		}
		return transferPath, nil
	}

	// transferUUID wasn't in p.transferRequests, so we need to look it up
	tr, err := p.stors.TransferRequestStor.GetTransferRequestByUUID(transferUUID)
	if err != nil {
		return &TransferPath{pathType: BadPathType}, err
	}

	p.transferRequests[transferUUID] = tr
	transferPath := &TransferPath{
		pathType:        ContextPathType,
		fullPath:        path,
		transferBase:    path,
		transferRequest: tr,
		stors:           p.stors,
	}

	return transferPath, nil
}

func (p *TransferPathParser) handleProjectPath(path string, pathParts []string) (Path, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// A fully formed path looks as follows:
	// pathParts[0] = ""
	// pathParts[1] = transfer-uuid
	// pathParts[2:] path to use for project path

	// If we are here then the transfer-uuid needs to exist in p.transferRequests. If it doesn't
	// then this is invalid. There is no need to go to the database because that look up should
	// have already happened.
	tr := p.transferRequests[pathParts[1]]
	if tr == nil {
		// This is an error because the transfer request uuid is not known
		return &TransferPath{pathType: BadPathType}, fmt.Errorf("unknown transfer request uuid %s", pathParts[1])
	}

	projectPathPieces := append([]string{"/"}, pathParts[2:]...)
	transferPath := &TransferPath{
		pathType:        ProjectPathType,
		projectPath:     filepath.Join(projectPathPieces...),
		fullPath:        path,
		transferBase:    filepath.Join("/", pathParts[1]), // create /transfer-uuid path
		transferRequest: tr,
		stors:           p.stors,
	}

	return transferPath, nil
}

func (p *TransferPathParser) Release(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pathParts := strings.Split(path, "/")
	// A fully formed path looks as follows:
	// pathParts[0] = ""
	// pathParts[1] = transfer-uuid
	// pathParts[2:] path to use for project path

	if len(pathParts) < 2 {
		// no transfer-uuid included, ignore
		return
	}

	transferUUID := pathParts[1]
	delete(p.transferRequests, transferUUID)
}
