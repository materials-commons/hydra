package fsstate

type FSState struct {
	ActivityTracker      *ActivityTracker
	TransferStateTracker *TransferStateTracker
	TransferRequestCache *TransferRequestCache
}

func NewFSState(tstateTracker *TransferStateTracker, trCache *TransferRequestCache, activityTracker *ActivityTracker) *FSState {
	return &FSState{
		TransferStateTracker: tstateTracker,
		TransferRequestCache: trCache,
		ActivityTracker:      activityTracker,
	}
}

func (s *FSState) GetCache() *TransferRequestCache {
	return s.TransferRequestCache
}

func (s *FSState) RemoveTransferRequestState(uuid string) {
	s.ActivityTracker.RemoveActivityFromTracking(uuid)
	s.TransferStateTracker.DeleteBase(uuid)
	s.TransferRequestCache.RemoveTransferRequestByUUID(uuid)
}
