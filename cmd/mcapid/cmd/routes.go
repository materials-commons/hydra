package cmd

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/materials-commons/hydra/pkg/mcapid/webapi"
	"github.com/materials-commons/hydra/pkg/mcapid/webapi/apimiddleware"
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

func setupExternalRoutes(e *echo.Echo, stors stor.Stors) {
	userCache := apimiddleware.NewAPIKeyCache(stors.UserStor)
	apikeyConfig := apimiddleware.APIKeyConfig{
		Skipper:         middleware.DefaultSkipper,
		Keyname:         "apikey",
		GetUserByAPIKey: userCache.GetUserByAPIKey,
	}

	projectAccessCache := apimiddleware.NewProjectAccessCache(stors.ProjectStor)
	projectAccessConfig := apimiddleware.ProjectAccessConfig{
		Skipper:            middleware.DefaultSkipper,
		HasAccessToProject: projectAccessCache.HasAccessToProject,
	}

	g := e.Group("/transfers")
	g.Use(apimiddleware.APIKeyAuth(apikeyConfig))
	g.Use(apimiddleware.ProjectAccessAuth(projectAccessConfig))

	transferUploadController := webapi.NewTransferUploadController(webapi.TransferUploadControllerConfig{
		ClientTransferStor:      stors.ClientTransferStor,
		FileStor:                stors.FileStor,
		TransferRequestStor:     stors.TransferRequestStor,
		TransferRequestFileStor: stors.TransferRequestFileStor,
		ClientTransferCache:     nil, // This needs to be initialized properly
	})
	g.POST("/uploads/start", transferUploadController.StartUpload)
	g.POST("/uploads/send", transferUploadController.SendUploadBytes)
	g.POST("/uploads/finish", transferUploadController.FinishUpload)
	g.POST("/uploads/cancel", transferUploadController.CancelUpload)
	g.GET("/uploads/status", transferUploadController.GetUploadStatus)
	g.GET("/uploads/verify", transferUploadController.GetVerifyStatus)

	transferDownloadController := webapi.NewTransferDownloadController(stors.ClientTransferStor)
	g.POST("/downloads/start", transferDownloadController.StartDownload)
	g.POST("/downloads/receive", transferDownloadController.ReceiveDownloadBytes)
	g.POST("/downloads/finish", transferDownloadController.FinishDownload)
	g.GET("/downloads/status", transferDownloadController.GetDownloadStatus)

	// Add the resumable upload controller for unlimited, restartable file uploads
	resumableUploadController := webapi.NewResumableUploadController(stors.FileStor)
	resumableUploadGroup := e.Group("/resumable-upload")
	resumableUploadGroup.Use(apimiddleware.APIKeyAuth(apikeyConfig))
	resumableUploadGroup.Use(apimiddleware.ProjectAccessAuth(projectAccessConfig))
	
	resumableUploadGroup.POST("/upload", resumableUploadController.Upload)
	resumableUploadGroup.POST("/finalize", resumableUploadController.FinalizeUpload)
	resumableUploadGroup.GET("/status", resumableUploadController.GetUploadStatus)
}
