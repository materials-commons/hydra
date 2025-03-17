package webapi

import (
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"net/http"
)

type TransferUploadController struct {
	clientTransferStor stor.ClientTransferStor
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

	clientTransfer, err := c.clientTransferStor.GetOrCreateClientTransferByPath(req.ClientUUID, req.ProjectID, 0, req.DestinationPath)

	_ = clientTransfer
	_ = err

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
