package webapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/apex/log"
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/globus"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/fsstate"
)

type TransferRequestsController struct {
	transferRequestStor stor.TransferRequestStor
	globusClient        *globus.Client
	fsState             *fsstate.FSState
}

func NewTransferRequestsController(globusClient *globus.Client, fsState *fsstate.FSState, transferRequestStor stor.TransferRequestStor) *TransferRequestsController {
	return &TransferRequestsController{transferRequestStor: transferRequestStor, globusClient: globusClient, fsState: fsState}
}

type TransferRequestStatus struct {
	transferRequestUUID string
	TransferRequest     *mcmodel.TransferRequest `json:"transfer_request"`
	ActivityCount       uint64                   `json:"activity_count"`
	LastActivityTime    string                   `json:"last_activity_time"`
	ActivityFound       bool                     `json:"activity_found"`
	Status              string                   `json:"status"`
}

const TransferRequestActive = "active"
const TransferRequestInactive = "inactive"
const TransferRequestStatusUnknown = "unknown"
const TransferRequestNoActivityState = "nostate"

func (c *TransferRequestsController) CloseAllInactiveTransferRequests(_ echo.Context) error {
	allTransferRequestsByStatus := c.getStatusForAllTransferRequests()

	var inactiveRequests []*TransferRequestStatus
	for _, tr := range allTransferRequestsByStatus {
		if tr.Status == TransferRequestInactive {
			inactiveRequests = append(inactiveRequests, tr)
			// TODO: GlobusTransfer isn't loaded, decide what we want to do here...
			_, _ = c.globusClient.DeleteEndpointACLRule(tr.TransferRequest.GlobusTransfer.GlobusEndpointID, tr.TransferRequest.GlobusTransfer.GlobusAclID)
		}
	}

	return nil
}

func (c *TransferRequestsController) CloseTransferRequest(ctx echo.Context) error {
	var req struct {
		TransferRequestUUID string `json:"transfer_request_uuid"`
	}

	if err := ctx.Bind(&req); err != nil {
		return err
	}

	tr, err := c.transferRequestStor.GetTransferRequestByUUID(req.TransferRequestUUID)
	if err != nil {
		return err
	}

	if tr.State != "closing" {
		return fmt.Errorf("transfer request state is '%s', should be 'closing'", tr.State)
	}

	c.fsState.TransferStateTracker.DeleteBase(tr.UUID)

	return nil
}

func (c *TransferRequestsController) GetStatusForTransferRequest(ctx echo.Context) error {
	transferUUID := ctx.Param("uuid")
	ac := c.fsState.ActivityTracker.GetActivityCounter(transferUUID)

	var (
		activity *TransferRequestStatus
		err      error
	)

	if ac == nil {
		activity.Status = TransferRequestNoActivityState
		activity.ActivityFound = false
		return ctx.JSON(http.StatusOK, activity)
	}

	activity.ActivityFound = true

	activity.TransferRequest, err = c.transferRequestStor.GetTransferRequestByUUID(activity.transferRequestUUID)
	if err != nil {
		activity.Status = TransferRequestStatusUnknown
	} else {
		activity.Status = TransferRequestActive
	}

	activity.LastActivityTime = ac.GetLastChanged().Format(time.RFC850)
	activity.ActivityCount = ac.GetActivityCount()

	return ctx.JSON(http.StatusOK, activity)
}

func (c *TransferRequestsController) IndexTransferRequestStatus(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, c.getStatusForAllTransferRequests())
}

func (c *TransferRequestsController) getStatusForAllTransferRequests() []*TransferRequestStatus {
	transferRequests := make(map[string]*TransferRequestStatus)

	// Get all transfer requests that have seen some activity
	c.fsState.ActivityTracker.ForEach(func(transferRequestUUID string, ac *fsstate.ActivityCounter) {
		activity := &TransferRequestStatus{
			transferRequestUUID: transferRequestUUID,
			ActivityCount:       ac.GetActivityCount(),
			LastActivityTime:    ac.GetLastChanged().Format(time.RFC850),
			ActivityFound:       true,
		}
		transferRequests[activity.transferRequestUUID] = activity
	})

	// For the active transfer requests retrieve the transfer request from the database
	var err error
	for _, activity := range transferRequests {
		activity.Status = TransferRequestActive
		activity.TransferRequest, err = c.transferRequestStor.GetTransferRequestByUUID(activity.transferRequestUUID)
		if err != nil {
			activity.Status = TransferRequestStatusUnknown
			log.Errorf("TransferRequestsController.IndexTransferRequestStatus TransferRequest %s: %s", activity.transferRequestUUID, err)
		}
	}

	// There may be transfer requests for which there has been no activity, lets gather those by
	// getting all transfer requests, filtering out active transfer requests, and then adding the
	// inactive transfer requests into the transferRequests map.
	allTransferRequests, err := c.transferRequestStor.ListTransferRequests()
	if err != nil {
		log.Errorf("TransferRequestsController.IndexTransferRequestStatus unable to retrieve all transfer requests: %s", err)
	}

	for _, transferRequest := range allTransferRequests {
		if _, ok := transferRequests[transferRequest.UUID]; !ok {
			// Didn't find this request so let's add it to hashmap
			tr := transferRequest
			activity := &TransferRequestStatus{
				TransferRequest:  &tr,
				LastActivityTime: "unknown",
				ActivityCount:    0,
				Status:           TransferRequestInactive,
			}
			transferRequests[transferRequest.UUID] = activity
		}
	}

	// We now have all transfer requests, so turn back into an array and return
	var trequests []*TransferRequestStatus
	for _, tr := range transferRequests {
		trequests = append(trequests, tr)
	}

	return trequests
}
