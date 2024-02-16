package cmd

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/materials-commons/hydra/pkg/config"
	"github.com/materials-commons/hydra/pkg/globus"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/fsstate"
	"github.com/materials-commons/hydra/pkg/mcfs/webapi"
)

type RouteDependencies struct {
	e            *echo.Echo
	config       config.Configer
	stors        *stor.Stors
	fsState      *fsstate.FSState
	globusClient *globus.Client
}

func setupRoutes(deps RouteDependencies) {
	deps.e.Use(middleware.Recover())
	g := deps.e.Group("/api")

	logController := webapi.NewLogController()
	g.POST("/set-logging-level", logController.SetLogLevelHandler)
	g.POST("/set-logging-output", logController.SetLogOutputHandler)
	g.POST("/set-logging", logController.SetLoggingHandler)
	g.GET("/show-logging", logController.ShowCurrentLoggingHandler)

	transferRequestsActivityController := webapi.NewTransferRequestsController(deps.globusClient, deps.fsState, deps.stors.TransferRequestStor)
	g.GET("/transfers", transferRequestsActivityController.IndexTransferRequestStatus)
	g.GET("/transfers/:uuid/status", transferRequestsActivityController.GetStatusForTransferRequest)
}
