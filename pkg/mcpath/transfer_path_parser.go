package mcpath

import (
	"sync"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type TransferPathParser struct {
	mu                  sync.Mutex
	transferRequests    map[string]*mcmodel.TransferRequest
	transferRequestStor stor.TransferRequestStor
}

func NewTransferPathParser(transferRequestStor stor.TransferRequestStor) Parser {
	return &TransferPathParser{
		transferRequests:    make(map[string]*mcmodel.TransferRequest),
		transferRequestStor: transferRequestStor,
	}
}

func (p *TransferPathParser) Parse(path string) (Path, error) {
	return nil, nil
}
