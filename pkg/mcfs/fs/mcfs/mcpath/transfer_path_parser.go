package mcpath

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/fsstate"
)

type TransferPathParser struct {
	trCache *fsstate.TransferRequestCache
	stors   *stor.Stors
}

func NewTransferPathParser(stors *stor.Stors, trCache *fsstate.TransferRequestCache) ParserReleaser {
	return &TransferPathParser{
		stors:   stors,
		trCache: trCache,
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
	tr, err := p.trCache.GetTransferRequestByUUID(transferUUID)
	if err != nil {
		return &TransferPath{pathType: BadPathType}, err
	}

	transferPath := &TransferPath{
		pathType:        ContextPathType,
		projectPath:     "/",
		fullPath:        path,
		transferBase:    path,
		transferRequest: tr,
		stors:           p.stors,
	}
	return transferPath, nil
}

func (p *TransferPathParser) handleProjectPath(path string, pathParts []string) (Path, error) {
	// A fully formed path looks as follows:
	// pathParts[0] = ""
	// pathParts[1] = transfer-uuid
	// pathParts[2:] path to use for project path

	// If we are here then the transfer-uuid needs to exist in p.transferRequests. If it doesn't
	// then this is invalid. There is no need to go to the database because that look up should
	// have already happened.
	tr, err := p.trCache.GetTransferRequestByUUID(pathParts[1])
	if err != nil {
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

// TODO: Can this be removed?
func (p *TransferPathParser) Release(path string) {
	pathParts := strings.Split(path, "/")
	// A fully formed path looks as follows:
	// pathParts[0] = ""
	// pathParts[1] = transfer-uuid
	// pathParts[2:] path to use for project path

	if len(pathParts) < 2 {
		// no transfer-uuid included, ignore
		return
	}

	//transferUUID := pathParts[1]
	//delete(p.transferRequests, transferUUID)
}
