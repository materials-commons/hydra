package mcapid

import (
	"errors"
	"sync"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

//
// Cache design
//
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
//     ErrNoClientTransfer
//     ErrNoMatchingClientTransferRequestFile
// API:
//    GetTransferRequestFileByPath(client_uuid, project_id, owner_id, path) (*mcmodel.TransferRequestFile, error)
//    GetTransferRequestFileByID(client_uuid, project_id, owner_id, transfer_request_file_id) (*mcmodel.TransferRequestFile, error)
//    WithWriteLockHeld(fn func(cache NoLockHeldClientTransferCache) (*mcmodel.TransferRequestFile, error)) (*mcmodel.TransferRequestFile, error)
//

var ErrNoClientTransfer = errors.New("no client transfer")
var ErrNoMatchingClientTransferRequestFile = errors.New("no matching client transfer file found")

type ClientTransferCache interface {
	GetTransferRequestFileByPath(clientUUID string, projectID int, ownerID int, path string) (*mcmodel.TransferRequestFile, error)
	GetTransferRequestFileByID(clientUUID string, projectID int, ownerID int, transferRequestFileID int) (*mcmodel.TransferRequestFile, error)
	WithWriteLockHeld(fn func(cache NoLockHeldClientTransferCache) (*mcmodel.TransferRequestFile, error)) (*mcmodel.TransferRequestFile, error)
}

type NoLockHeldClientTransferCache interface {
	InsertClientTransferNoLockHeld(clientUUID string, cf *mcmodel.ClientTransfer)
	InsertTransferRequestFileNoLockHeld(clientUUID string, trf *mcmodel.TransferRequestFile) error
	GetTransferRequestFileByPathNoLockHeld(clientUUID string, projectID int, ownerID int, path string) (*mcmodel.TransferRequestFile, error)
	GetTransferRequestFileByIDNoLockHeld(clientUUID string, projectID int, ownerID int, transferRequestFileId int) (*mcmodel.TransferRequestFile, error)
}

// A clientTransferEntry holds all the state for a client tranfer, including the underlying mcmodel.ClientTransfer as
// well as all the mcmodel.TransferRequestFiles associated with that transfer.
type clientTransferEntry struct {
	clientTransfer       *mcmodel.ClientTransfer
	transferRequestFiles []*mcmodel.TransferRequestFile
}

type ClientTransferCacheI struct {
	clientTransferEntriesMap map[string][]*clientTransferEntry
	mu                       sync.RWMutex
}

func NewClientTransferCache() *ClientTransferCacheI {
	return &ClientTransferCacheI{
		clientTransferEntriesMap: make(map[string][]*clientTransferEntry),
	}
}

func (c *ClientTransferCacheI) GetTransferRequestFileByPath(clientUUID string, projectID int, ownerID int, path string) (*mcmodel.TransferRequestFile, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.GetTransferRequestFileByPathNoLockHeld(clientUUID, projectID, ownerID, path)
}

func (c *ClientTransferCacheI) GetTransferRequestFileByID(clientUUID string, projectID int, ownerID int, transferRequestFileID int) (*mcmodel.TransferRequestFile, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.GetTransferRequestFileByIDNoLockHeld(clientUUID, projectID, ownerID, transferRequestFileID)
}

func (c *ClientTransferCacheI) WithWriteLockHeld(fn func(cache NoLockHeldClientTransferCache) (*mcmodel.TransferRequestFile, error)) (*mcmodel.TransferRequestFile, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return fn(c)
}

func (c *ClientTransferCacheI) InsertClientTransferNoLockHeld(clientUUID string, cf *mcmodel.ClientTransfer) {
	clientTransferEntry := makeClientTransferEntry(cf)
	c.clientTransferEntriesMap[clientUUID] = append(c.clientTransferEntriesMap[clientUUID], clientTransferEntry)
}

func makeClientTransferEntry(cf *mcmodel.ClientTransfer) *clientTransferEntry {
	return &clientTransferEntry{
		clientTransfer: cf,
	}
}

func (c *ClientTransferCacheI) InsertTransferRequestFileNoLockHeld(clientUUID string, trf *mcmodel.TransferRequestFile) error {
	matchingClientTransfer, err := c.getClientTransferEntryNoLockHeld(clientUUID, trf.ProjectID, trf.OwnerID)
	if err != nil {
		return err
	}

	matchingClientTransfer.transferRequestFiles = append(matchingClientTransfer.transferRequestFiles, trf)

	return nil
}

func (c *ClientTransferCacheI) GetTransferRequestFileByPathNoLockHeld(clientUUID string, projectID int, ownerID int, path string) (*mcmodel.TransferRequestFile, error) {
	matchingClientTransfer, err := c.getClientTransferEntryNoLockHeld(clientUUID, projectID, ownerID)
	if err != nil {
		return nil, err
	}

	// Found a matching client transfer, so now look for a matching TransferRequestFile by path
	for _, transferRequestFile := range matchingClientTransfer.transferRequestFiles {
		if transferRequestFile.Path == path {
			return transferRequestFile, nil
		}
	}

	// No match found
	return nil, ErrNoMatchingClientTransferRequestFile
}

func (c *ClientTransferCacheI) GetTransferRequestFileByIDNoLockHeld(clientUUID string, projectID int, ownerID int, transferRequestFileID int) (*mcmodel.TransferRequestFile, error) {
	matchingClientTransferEntry, err := c.getClientTransferEntryNoLockHeld(clientUUID, projectID, ownerID)
	if err != nil {
		return nil, err
	}

	// Found a matching client transfer, so now look for a matching TransferRequestFile by path
	for _, transferRequestFile := range matchingClientTransferEntry.transferRequestFiles {
		if transferRequestFile.ID == transferRequestFileID {
			return transferRequestFile, nil
		}
	}

	// No match found
	return nil, ErrNoMatchingClientTransferRequestFile
}

func (c *ClientTransferCacheI) getClientTransferEntryNoLockHeld(clientUUID string, projectID int, ownerID int) (*clientTransferEntry, error) {
	// Check if there are any client transfers associated with the ClientUUID
	clientTransferEntries, ok := c.clientTransferEntriesMap[clientUUID]
	if !ok {
		return nil, ErrNoClientTransfer
	}

	// Check if any of the client transfers associated with ClientUUID match the projectID and OwnerID
	for _, clientTransfer := range clientTransferEntries {
		if clientTransferEntryMatches(projectID, ownerID, clientTransfer) {
			return clientTransfer, nil
		}
	}

	// No client transfers matched
	return nil, ErrNoClientTransfer
}

func clientTransferEntryMatches(projectID int, ownerID int, clientTransferEntry *clientTransferEntry) bool {
	if clientTransferEntry.clientTransfer.ProjectID != projectID {
		return false
	}

	if clientTransferEntry.clientTransfer.OwnerID != ownerID {
		return false
	}

	return true
}
