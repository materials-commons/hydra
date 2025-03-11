package webapi

import (
	"github.com/labstack/echo/v4"
)

type TransferDownloadController struct {
}

func NewTransferDownloadController() *TransferDownloadController {
	return &TransferDownloadController{}
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
