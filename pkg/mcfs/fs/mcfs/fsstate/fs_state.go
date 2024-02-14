package fsstate

type FSState struct {
	ActivityCounterMonitor *ActivityCounterMonitor
	TransferStateTracker   *TransferStateTracker
	TransferRequestCache   *TransferRequestCache
}

func NewFSState(tstateTracker *TransferStateTracker, trCache *TransferRequestCache, acm *ActivityCounterMonitor) *FSState {
	return &FSState{
		TransferStateTracker:   tstateTracker,
		TransferRequestCache:   trCache,
		ActivityCounterMonitor: acm,
	}
}

func (s *FSState) GetCache() *TransferRequestCache {
	return s.TransferRequestCache
}

func (s *FSState) RemoveTransferRequestState(uuid string) {

}
