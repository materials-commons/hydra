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
	fsState                   *fsstate.FSState
	allowedInactivityDuration time.Duration
	closeTransferHandler      CloseActivityHandlerFN
	transfersToProcess        []string
	alreadySeenTasks          map[string]time.Time
}

func NewTransferMonitor(optFNs ...TransferMonitorOptionFN) *TransferMonitor {
	tm := &TransferMonitor{}

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
		m.checkAndCleanupInactiveTransfers()
		select {
		case <-c.Done():
			break
		case <-time.After(20 * time.Second):
		}
	}
}

func (m *TransferMonitor) checkAndCleanupInactiveTransfers() {
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

			// If we are here then the transfer has been inactive for at least 20 seconds.
			// Let's add the transfer to the list of transfers to check on globus to see
			// if it's done.
			m.transfersToProcess = append(m.transfersToProcess, transferUUID)

		default:
			activityCounter.SetLastChangedAt(now)
			activityCounter.SetLastSeenActivityCount(currentActivityCount)
		}

		return nil
	})

	if len(m.transfersToProcess) != 0 {
		m.processSuccessfulGlobusTasks()
	}
}

func (m *TransferMonitor) processSuccessfulGlobusTasks() {
	// Remove old tasks we've seen before so the list doesn't continuously grow.
	m.removeOldTasks()

	// Build a filter to get all successful tasks that completed in the last hour
	lastHour := time.Now().Add(time.Hour).Format(time.RFC3339)
	taskFilter := map[string]string{
		"filter_completion_time": lastHour,
		"filter_status":          "SUCCEEDED",
	}

	tasks, err := m.globusClient.GetEndpointTaskList(m.globusEndpointID, taskFilter)
	if err != nil {
		clog.Global().Errorf("globus.GetEndpointTaskList returned the following error: %s - %#v", err, m.globusClient.GetGlobusErrorResponse())
	}

	for _, task := range tasks.Tasks {
		_, ok := m.alreadySeenTasks[task.TaskID]
		if ok {
			continue
		}

		transfers, err := m.globusClient.GetTaskSuccessfulTransfers(task.TaskID, 0)
		switch {
		case err != nil:
			clog.Global().Errorf("globus.GetTaskSuccessfulTransfers(%s) returned error %s - %#v", task.TaskID, err, m.globusClient.GetGlobusErrorResponse())
			continue

		case len(transfers.Transfers) == 0:
			continue

		default:
			// No files transferred in this request

		}
	}

	m.transfersToProcess = nil
}

func (m *TransferMonitor) removeOldTasks() {
	// First remove all tasks that already been seen and are over
	// 1 hour old
	now := time.Now()
	for k, v := range m.alreadySeenTasks {
		allowedAge := v.Add(time.Hour)
		if now.After(allowedAge) {
			// if the time now is more than 1 hour later than the
			// already seen tasks age + 1 hour, then delete it
			delete(m.alreadySeenTasks, k)
		}
	}
}
