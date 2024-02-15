package monitor

import (
	"context"
	"time"

	"github.com/materials-commons/hydra/pkg/clog"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/fsstate"
)

type CloseActivityHandlerFN func(activityKey string)

type TransferMonitorOptionFN func(*TransferMonitor)

type TransferMonitor struct {
	transferRequestStor       stor.TransferRequestStor
	fsState                   *fsstate.FSState
	allowedInactivityDuration time.Duration
	closeTransferHandler      CloseActivityHandlerFN
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
			}

		default:
			activityCounter.SetLastChangedAt(now)
			activityCounter.SetLastSeenActivityCount(currentActivityCount)
		}

		return nil
	})
}
