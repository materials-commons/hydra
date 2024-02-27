package monitor

import (
	"context"
	"time"

	"github.com/materials-commons/hydra/pkg/clog"
	"github.com/materials-commons/hydra/pkg/globus"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/fsstate"
)

type CloseActivityHandlerFN func(activityKey string)

type TransferMonitorOptionFN func(*TransferMonitor)

type TransferMonitor struct {
	globusClient              *globus.Client
	globusEndpointID          string
	transferRequestStor       stor.TransferRequestStor
	globusTransferStor        stor.GlobusTransferStor
	fsState                   *fsstate.FSState
	allowedInactivityDuration time.Duration
	globusTaskWindowDuration  time.Duration
	closeTransferHandler      CloseActivityHandlerFN
	transfersToProcess        []string
	alreadySeenTasks          map[string]time.Time
}

func NewTransferMonitor(optFNs ...TransferMonitorOptionFN) *TransferMonitor {
	// Set defaults
	tm := &TransferMonitor{
		globusTaskWindowDuration:  time.Hour,
		allowedInactivityDuration: time.Hour * 24 * 7,
	}

	for _, optfn := range optFNs {
		optfn(tm)
	}

	return tm
}

func WithTransferRequestStor(trStor stor.TransferRequestStor) TransferMonitorOptionFN {
	return func(m *TransferMonitor) {
		m.transferRequestStor = trStor
	}
}

func WithGlobuTransferStor(gtStor stor.GlobusTransferStor) TransferMonitorOptionFN {
	return func(m *TransferMonitor) {
		m.globusTransferStor = gtStor
	}
}

func WithFSState(fsState *fsstate.FSState) TransferMonitorOptionFN {
	return func(m *TransferMonitor) {
		m.fsState = fsState
	}
}

func WithAllowedInactivityDuration(inactivityDuration time.Duration) TransferMonitorOptionFN {
	return func(m *TransferMonitor) {
		m.allowedInactivityDuration = inactivityDuration
	}
}

func WithGlobusTaskWindowDuration(taskWindowDuration time.Duration) TransferMonitorOptionFN {
	return func(m *TransferMonitor) {
		m.globusTaskWindowDuration = taskWindowDuration
	}
}

func WithCloseTransferHandler(f CloseActivityHandlerFN) TransferMonitorOptionFN {
	return func(m *TransferMonitor) {
		m.closeTransferHandler = f
	}
}

func WithGlobusClient(gc *globus.Client) TransferMonitorOptionFN {
	return func(m *TransferMonitor) {
		m.globusClient = gc
	}
}

func WithGlobusEndpointID(endpointID string) TransferMonitorOptionFN {
	return func(m *TransferMonitor) {
		m.globusEndpointID = endpointID
	}
}

func (m *TransferMonitor) Run(c context.Context) {
	for {
		m.checkAndCleanupFinishedTransfers()
		m.checkAndHandleInactiveTransfers()
		select {
		case <-c.Done():
			break
		case <-time.After(20 * time.Second):
		}
	}
}

// checkAndCleanupFinishedTransfers cleans up the state for all finished transfers. This means all cleaning up
// transfer_requests in the database and moving the files they refer to into the current files for a project.
// The algorithm for determining if a transfer request has a number of limitations due to globus that it has
// to work within. These limitations include:
//  1. We can only find the path for files that have successfully transferred.
//  2. The only thing an active transfer will tell us is the globus user that has an active request. There isn't
//     other information we can use to determine which Materials Commons TransferRequest directory this transfer
//     is associated with.
//
// Combined these make determining completed transfers difficult. The file system knows the following:
//
//  1. It knows what transfers have been active. This is because a globus transfer is limited to a directory
//     that looks like /__transfer/<transfer-request-uuid>/<... rest of path>. The file system can track activity
//     for that particular transfer by associating it with the <transfer-request-uuid>. This state is kept in
//     the ActivityTracker.
//
//  2. The filesystem knows when a file was last closed. Just because the file was closed doesn't mean that
//     globus is done writing to it. This is because Globus might open and close the file multiple times during
//     a transfer. That said it is, however, a strong indicator that Globus is done.
//
// So to determine if Globus is done with a transfer we do the following:
//
//	Every XX seconds we wake up check the list of active transfers. This check is not very precise
//	because Globus will only tell us which user has an active transfer, not where the transfer is
//	going to. So, in order to process a users finished transfers that user must have no active
//	transfers.
//
//	This limitation has the following implication: If a user has two projects they are doing transfers
//	to (so two active TransferRequests), we cannot determine of these transfers are completed, and which
//	are still active. So, all of a users transfers must complete before we can process their files. We
//	also give the user the ability to mark a transfer as complete. Since the user has this knowledge,
//	this gives them the option to mark a transfer as done so we can process the files faster.
//
//	Once a user has no active transfers we lock writes to the file system for the users transfers. Then
//	we check to make sure no writes got through while we were performing the lock. Assuming that is the
//	case then we process all their files and release the lock.
//
//	If a write did get through, that means there is now an active transfer. We release the lock and
//	skip processing the user's files.
//
//	Lastly we need to clean up globus state, release ACLs, etc... There is a timing issue here because
//	we may decide to perform the cleanup just as the user decides to do a transfer. To handle this
//	case we send the user an email letting them know that they need to initiate a new globus transfer.
//	This may still cause some support issues, but the window is small.
func (m *TransferMonitor) checkAndCleanupFinishedTransfers() {
	activeTasksFilter := map[string]string{
		"filter_status": "ACTIVE",
	}

	activeTasksList, err := m.globusClient.GetEndpointTaskList(m.globusEndpointID, activeTasksFilter)
	if err != nil {
		clog.Global().Errorf("globus.GetEndpointTaskList returned the following error getting successful tasks: %s - %#v",
			err, m.globusClient.GetGlobusErrorResponse())
		return
	}

	// Now that we have a list of active tasks, go through the tasks and identify all the users that have
	// an active transfer (according to Globus). Any TransferRequest that is owned by one of these users
	// will be ignored. All other transfer requests are candidates for processing.

	// A map of userid to nothing. We use a map to quickly look up user ids.
	usersWithActiveTransfers := make(map[int]struct{})
	for _, task := range activeTasksList.Tasks {
		userID, err := m.globusOwnerID2MCUserID(task.OwnerID)
		if err != nil {
			// what do we do in this case?
		}
		usersWithActiveTransfers[userID] = struct{}{}
	}

	transferRequests, err := m.transferRequestStor.ListTransferRequests()
	if err != nil {
		// log err
		return
	}

	for _, tr := range transferRequests {
		if _, ok := usersWithActiveTransfers[tr.OwnerID]; ok {
			// This transfer request is owned by a user who has an active transfer, so skip it
			continue
		}

		if !m.lockAndCheckInactiveAfterLockTransferRequest(&tr) {
			// If this failed then we asked the file system to lock activity, but between
			// deciding to lock, and locking a write() got through, so skip processing
			// this transfer request
			continue
		}

		m.processTransferRequestFiles(&tr)
		m.unlockTransferRequest(&tr)
	}
}

func (m *TransferMonitor) globusOwnerID2MCUserID(globusUserUUID string) (int, error) {
	globusTransfer, err := m.globusTransferStor.GetGlobusTransferByGlobusIdentityID(globusUserUUID)
	return globusTransfer.OwnerID, err
}

func (m *TransferMonitor) lockAndCheckInactiveAfterLockTransferRequest(tr *mcmodel.TransferRequest) bool {
	activityCounter := m.fsState.ActivityTracker.GetActivityCounter(tr.UUID)
	if activityCounter == nil {
		return false
	}

	currentCount := activityCounter.GetActivityCount()
	activityCounter.PreventWrites()

	// Give system a chance for any in process writes to complete and update
	// the counter.
	time.Sleep(10 * time.Millisecond)

	// Check that something didn't sneak through
	newCount := activityCounter.GetActivityCount()
	if newCount == currentCount {
		// No writes came through after setting PreventWrites()
		return true
	}

	// Write(s) got through, unlock and return false
	activityCounter.AllowWrites()
	return false
}

func (m *TransferMonitor) processTransferRequestFiles(tr *mcmodel.TransferRequest) {

}

func (m *TransferMonitor) unlockTransferRequest(tr *mcmodel.TransferRequest) {
	activityCounter := m.fsState.ActivityTracker.GetActivityCounter(tr.UUID)
	if activityCounter == nil {
		return
	}

	activityCounter.AllowWrites()
}

func (m *TransferMonitor) checkAndHandleInactiveTransfers() {
	m.fsState.ActivityTracker.ForEach(func(transferUUID string, activityCounter *fsstate.ActivityCounter) error {
		var (
			err error
			tr  *mcmodel.TransferRequest
		)
		// First check if the transfer request has been closed
		tr, err = m.transferRequestStor.GetTransferRequestByUUID(transferUUID)

		switch {
		case err == nil:
			// Found the transfer request. Let's check its state.
			if tr.State == "closed" {
				// Found the transfer request and its marked as closed. In that case clean up
				// the state and return.
				m.closeTransferHandler(transferUUID)
				return nil
			}
		case stor.IsRecordNotFound(err):
			// Couldn't find the record in the database. That means it was deleted externally. In
			// that case we need to clean up the state.
			m.closeTransferHandler(transferUUID)
			return nil

		case err != nil:
			// err != nil and err != RecordNotFound (because we check RecordNotFound above, don't change the
			// order of the err checks!).
			//
			// Problem connecting to the database. Log error and stop all checking. This will cause us
			// to wait until the next time this checker runs before accessing the database, giving the
			// system time to recover from failures such as the database being (hopefully temporarily)
			// unavailable.
			clog.Global().Errorf("Failed retrieving transfer request with uuid %s from the database with error: %s", transferUUID, err)
			return err
		}

		// If we are here, then the transfer request was found, but its state != "closed". When this
		// happens we need to check how active the transfer has been. If it exceeds the duration for
		// inactivity then we need to close it. If it hasn't exceeded the activity limit then we
		// record its current activity level by calling SetLastSeenActivityCount().
		//
		// We check activity as follows. There are two counters
		//    currentActivityCount which tracks each read/write
		//    lastSeenActivityCount which is currentActivityCount from the last time we checked.
		//
		// if currentActivityCount == lastSeenActivityCount then there has been no
		// activity since the last check. In that case we check how long this
		// transfer has been inactive, and if it has exceeded the inactivity duration
		// then we close it.
		//
		// if currentActivityCount != lastSeenActivityCount then this transfer has seen
		// activity since the last check. So we set lastSeenActivityCount to
		// currentActivityCount, and set lastChangedAt to time.Now(). This will reset
		// the inactivity time and reset the inactive duration check.

		currentActivityCount := activityCounter.GetActivityCount()
		lastSeenActivityCount := activityCounter.GetLastSeenActivityCount()
		now := time.Now()

		switch {
		case currentActivityCount == lastSeenActivityCount:
			lastChangedAt := activityCounter.GetLastChangedAt()
			allowedInactive := lastChangedAt.Add(m.allowedInactivityDuration)
			if now.After(allowedInactive) {
				m.closeTransferHandler(transferUUID)
				return nil
			}

		default:
			activityCounter.SetLastChangedAt(now)
			activityCounter.SetLastSeenActivityCount(currentActivityCount)
		}

		return nil
	})

}
