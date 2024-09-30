package webapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcapid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type FolderController struct {
	fileStor stor.FileStor
}

func NewFolderController(fileStor stor.FileStor) *FolderController {
	return &FolderController{fileStor: fileStor}
}

func (c *FolderController) GetOrCreateFolderPath(ctx echo.Context) error {
	var (
		req struct {
			ProjectID int    `json:"project_id"`
			OwnerID   int    `json:"owner_id"`
			Path      string `json:"path"`
		}
		folder *mcmodel.File
		err    error
	)

	if err = ctx.Bind(&req); err != nil {
		return err
	}

	err = mcapid.WithProjectMutex(req.ProjectID, func() error {
		folder, err = c.fileStor.GetOrCreateDirPath(req.ProjectID, req.OwnerID, req.Path)
		return err
	})

	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK, folder)
}

func (c *FolderController) GetOrCreateFolder(ctx echo.Context) error {
	var (
		req struct {
			ParentDirID int    `json:"parent_dir_id"`
			ProjectID   int    `json:"project_id"`
			OwnerID     int    `json:"owner_id"`
			Path        string `json:"path"`
			Name        string `json:"name"`
		}
		folder *mcmodel.File
		err    error
	)

	if err := ctx.Bind(&req); err != nil {
		return err
	}
	err = mcapid.WithProjectMutex(req.ProjectID, func() error {
		folder, err = c.fileStor.CreateDirectory(req.ParentDirID, req.ProjectID, req.OwnerID, req.Path, req.Name)
		return err
	})

	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK, folder)
}
