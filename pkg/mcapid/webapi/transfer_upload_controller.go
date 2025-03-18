package webapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcapid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type TransferUploadController struct {
	clientTransferStor  stor.ClientTransferStor
	clientTransferCache mcapid.ClientTransferCache
}

func NewTransferUploadController(clientTransferStor stor.ClientTransferStor) *TransferUploadController {
	return &TransferUploadController{clientTransferStor: clientTransferStor}
}

func (c *TransferUploadController) StartUpload(ctx echo.Context) error {
	var req struct {
		DestinationPath string `json:"destination_path"`
		ProjectID       int    `json:"project_id"`
		Size            uint64 `json:"size"`
		ClientUUID      string `json:"client_uuid"`
		Checksum        string `json:"checksum"`
		ClientModTime   string `json:"client_mod_time"`
	}

	if err := ctx.Bind(&req); err != nil {
		return ctx.NoContent(http.StatusBadRequest)
	}

	user := ctx.Get("user").(*mcmodel.User)
	_ = user
	transferRequestFile, err := c.clientTransferCache.GetOrCreateClientTransferRequestFileByPath(req.ClientUUID, req.ProjectID, req.DestinationPath, user.ID, nil)
	_ = transferRequestFile
	_ = err

	//
	//clientTransfer, transferRequestFile, err := c.clientTransferStor.GetOrCreateClientTransferByPath(req.ClientUUID, req.ProjectID, 0, req.DestinationPath)
	//
	//if err != nil {
	//	return err
	//}

	return nil
}

func (c *TransferUploadController) SendUploadBytes(ctx echo.Context) error {
	return nil
}

func (c *TransferUploadController) FinishUpload(ctx echo.Context) error {
	return nil
}

func (c *TransferUploadController) CancelUpload(ctx echo.Context) error {
	return nil
}

func (c *TransferUploadController) GetUploadStatus(ctx echo.Context) error {
	return nil
}

func (c *TransferUploadController) GetVerifyStatus(ctx echo.Context) error {
	return nil
}
