package cmd

import (
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcapid/webapi"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type RouteOpts struct {
	fileStor stor.FileStor
}

func setupRoutes(e *echo.Echo, opts RouteOpts) {
	g := e.Group("/api")

	folderController := webapi.NewFolderController(opts.fileStor)
	g.POST("/folders", folderController.GetOrCreateFolder)
}
