package fsstate

import (
	"sync"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type TransferRequestCache struct {
	cache               sync.Map
	transferRequestStor stor.TransferRequestStor
}

func NewTransferRequestCache(trStor stor.TransferRequestStor) *TransferRequestCache {
	return &TransferRequestCache{transferRequestStor: trStor}
}

func (c *TransferRequestCache) GetTransferRequestByUUID(uuid string) (*mcmodel.TransferRequest, error) {
	tr, ok := c.cache.Load(uuid)
	if ok {
		return tr.(*mcmodel.TransferRequest), nil
	}

	retrieved, err := c.transferRequestStor.GetTransferRequestByUUID(uuid)
	if err != nil {
		return nil, err
	}

	tr, _ = c.cache.LoadOrStore(uuid, retrieved)
	return tr.(*mcmodel.TransferRequest), nil
}

func (c *TransferRequestCache) RemoveTransferRequestByUUID(uuid string) {
	c.cache.Delete(uuid)
}
