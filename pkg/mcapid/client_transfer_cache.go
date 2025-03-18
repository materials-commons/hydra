package mcapid

import (
	"sync"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type ClientTransferCache interface {
	GetOrCreateClientTransferRequestFileByPath(clientUUID string, projectID int, path string, ownerID int, fn CreateClientTransferFN) (*mcmodel.TransferRequestFile, error)
}

type clientTransfer struct {
	clientTransfer            *mcmodel.ClientTransfer
	transfersByTransferFileId sync.Map // map[id]*TransferRequestFile
	transfersByPath           sync.Map // map[path]*TransferRequestFile
}

// Cache design
// Requirements: Need to be able to look up a client request quickly. A request will consist of
// The client_uuid, project_id, owner_id, and one of [path] or [transfer_request_file_id], and optionally
// [transfer_request_id]
// The data structure will have a map of key: client_uuid, <list?> of clientTransfers
// A clientTransfer contains the mcmodel.ClientTransfer, we can iterate through the list of clientTransfers looking
// for the ClientTransfer that matches project_id, owner_id and if present the transfer_request_id
// Once that is found, we need to find or create the TransferRequestFile, we can look either based on path
// or based on transfer_request_file_id. This will be stored (for now) in a list of TransferRequestFiles. We will
// iterate through the list looking for either the transfer_request_file_id, or the Path.
//
// Creation: if a client_uuid doesn't exist then we need to create one containing a clientTransfer (ClientTransfer)
// create the TransferRequest, and create the first TransferRequestFile.
// if a client_uuid does exist, we may still need to create the ClientTransfer, TransferRequest and TransferRequestFile
// if there is nothing matching that user_id and project_id.
// If there is a matching ClientTransfer/TransferRequest, then we need to only check if the TransferRequestFile
// exists, and if not create it.
//
// Note: The initial design of this cache assumes that it is sparsely populated. For a particular client_uuid, that
// it will not have a lot of ClientTransfers so looping through them will be very inexpensive. The same goes when
// looking for TransferRequestFiles, that for a project_id/owner_id, there will only be a couple of files (at most)
// in flight and that finding the matching request will be cheap.
//
// One goal is to hold locks for as little time as possible and to use read locks as much as possible. The API
// design will reflect this by giving calls that grab a read lock to find data. Then we will provide an API
// that will hold a write lock and a series of calls that are meant to be used in the context of that write lock
// that can search and insert without grabbing a lock. Because the checks will go read lock to write lock, the
// caller will be responsible for checking that in between read lock, drop read lock, get write lock, that something
// didn't sneak in. That means the call will need to do all the existence checks again, but they MUST use the API
// that doesn't grab a lock.
//
// Calls that don't hold a lock will have WithoutLock appended to them.
//
// Errors:
//     ErrNoSuchClientUUID
//     ErrNoClientTransfer
//     ErrNoMatchingClientTransferRequestFile
// API:
//    GetTransferRequestFileByPath(client_uuid, project_id, owner_id, path) (*mcmodel.TransferRequestFile, error)
//    GetTransferRequestFileByID(client_uuid, project_id, owner_id, transfer_request_file_id) (*mcmodel.TransferRequestFile, error)
//    WithWriteLockHeld(func(clientTransferInserterFn, clientTransferRequestFileInserterFn, checkByPathFn, checkByIDFn) error) (*mcmodel.TransferRequestFile, error)

type ClientTransferCacheI struct {
	// clientTransfersMap is a map[client_uuid]*clientTransfer
	// where client_uuid is the (string) client_uuid field stored in a ClientTransfer
	clientTransfersMap      map[string]*clientTransfer
	mu                      sync.Mutex
	clientTransferStor      stor.ClientTransferStor
	transferRequestFileStor stor.TransferRequestFileStor
}

type CreateClientTransferFN func() (*mcmodel.ClientTransfer, error)

func NewClientTransferCache(clientTransferStor stor.ClientTransferStor, transferRequestFileStor stor.TransferRequestFileStor) *ClientTransferCacheI {
	return &ClientTransferCacheI{clientTransferStor: clientTransferStor, transferRequestFileStor: transferRequestFileStor}
}

func (c *ClientTransferCacheI) GetOrCreateClientTransfer(clientUUID string, fn CreateClientTransferFN) (*mcmodel.ClientTransfer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ct, ok := c.clientTransfersMap[clientUUID]; ok {
		return ct.clientTransfer, nil
	}

	ct, err := fn()
	if err != nil {
		return nil, err
	}

	c.clientTransfersMap[clientUUID] = &clientTransfer{clientTransfer: ct}

	return ct, nil
}

func (c *ClientTransferCacheI) GetOrCreateTransferRequestFileByPath(clientUUID string, projectID int, path string) (*mcmodel.TransferRequestFile, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ct, ok := c.clientTransfersMap[clientUUID]; !ok {
		_ = ct
	}

	return nil, nil
}
