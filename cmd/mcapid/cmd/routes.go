package cmd

import (
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcapid/webapi"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type RouteOpts struct {
	fileStor stor.FileStor
}

func setupInternalRoutes(e *echo.Echo, opts RouteOpts) {
	g := e.Group("/api")

	folderController := webapi.NewFolderController(opts.fileStor)

	g.POST("/folders", folderController.GetOrCreateFolder)
	g.POST("/folders/by-path", folderController.GetOrCreateFolderPath)

}

func setupExternalRoutes(e *echo.Echo, clientTransferStor stor.ClientTransferStor) {
	g := e.Group("/transfers")

	transferUploadController := webapi.NewTransferUploadController(clientTransferStor)
	g.POST("/uploads/start", transferUploadController.StartUpload)
	g.POST("/uploads/send", transferUploadController.SendUploadBytes)
	g.POST("/uploads/finish", transferUploadController.FinishUpload)
	g.POST("/uploads/cancel", transferUploadController.CancelUpload)
	g.GET("/uploads/status", transferUploadController.GetUploadStatus)
	g.GET("/uploads/verify", transferUploadController.GetVerifyStatus)

	transferDownloadController := webapi.NewTransferDownloadController(clientTransferStor)
	g.POST("/downloads/start", transferDownloadController.StartDownload)
	g.POST("/downloads/receive", transferDownloadController.ReceiveDownloadBytes)
	g.POST("/downloads/finish", transferDownloadController.FinishDownload)
	g.GET("/downloads/status", transferDownloadController.GetDownloadStatus)
}
