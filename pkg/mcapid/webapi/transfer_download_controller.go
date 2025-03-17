package webapi

import (
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type TransferDownloadController struct {
	clientTransferStor stor.ClientTransferStor
}

func NewTransferDownloadController(clientTransferStor stor.ClientTransferStor) *TransferDownloadController {
	return &TransferDownloadController{clientTransferStor: clientTransferStor}
}

func (c *TransferDownloadController) StartDownload(ctx echo.Context) error {
	return nil
}

func (c *TransferDownloadController) ReceiveDownloadBytes(ctx echo.Context) error {
	return nil
}

func (c *TransferDownloadController) FinishDownload(ctx echo.Context) error {
	return nil
}

func (c *TransferDownloadController) GetDownloadStatus(ctx echo.Context) error {
	return nil
}
