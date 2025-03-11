package webapi

import (
	"github.com/labstack/echo/v4"
)

type TransferUploadController struct {
}

func NewTransferUploadController() *TransferUploadController {
	return &TransferUploadController{}
}

func (c *TransferUploadController) StartUpload(ctx echo.Context) error {
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
