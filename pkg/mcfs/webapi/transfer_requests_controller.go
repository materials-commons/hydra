package webapi

import (
	"net/http"
	"time"

	"github.com/apex/log"
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs"
)

type TransferRequestsController struct {
	activity            *mcfs.ActivityCounterMonitor
	transferRequestStor stor.TransferRequestStor
}

func NewTransferRequestsController(activity *mcfs.ActivityCounterMonitor, transferRequestStor stor.TransferRequestStor) *TransferRequestsController {
	return &TransferRequestsController{activity: activity, transferRequestStor: transferRequestStor}
}

type TransferRequestStatus struct {
	transferRequestUUID string
	TransferRequest     *mcmodel.TransferRequest `json:"transfer_request"`
	ActivityCount       int64                    `json:"activity_count"`
	LastActivityTime    string                   `json:"last_activity_time"`
	Status              string                   `json:"status"`
}

const TransferRequestActive = "active"
const TransferRequestInactive = "inactive"
const TransferRequestStatusUnknown = "unknown"

func (c *TransferRequestsController) ListTransferRequestStatus(ctx echo.Context) error {
	transferRequests := make(map[string]*TransferRequestStatus)

	// Get all transfer requests that have seen some activity
	c.activity.ForEach(func(transferRequestUUID string, ac *mcfs.ActivityCounter) {
		activity := &TransferRequestStatus{
			transferRequestUUID: transferRequestUUID,
			ActivityCount:       ac.LastSeenActivityCount,
			LastActivityTime:    ac.LastChanged.Format(time.RFC850),
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
			log.Errorf("TransferRequestsController.ListTransferRequestStatus TransferRequest %s: %s", activity.transferRequestUUID, err)
		}
	}

	// There may be transfer requests for which there has been no activity, lets gather those by
	// get all transfer requests, filtering out active transfer requests, and then adding the
	// inactive transfer requests into the transferRequests map
	allTransferRequests, err := c.transferRequestStor.ListTransferRequests()
	if err != nil {
		log.Errorf("TransferRequestsController.ListTransferRequestStatus unable to retrieve all transfer requests: %s", err)
	}

	for _, transferRequest := range allTransferRequests {
		if _, ok := transferRequests[transferRequest.UUID]; !ok {
			// Didn't find this request so let's add it to hashmap
			activity := &TransferRequestStatus{
				TransferRequest:  &transferRequest,
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

	return ctx.JSON(http.StatusOK, trequests)
}
