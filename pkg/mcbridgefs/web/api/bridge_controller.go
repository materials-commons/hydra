package api

import (
	"fmt"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
	"sync"

	"github.com/apex/log"
	"github.com/labstack/echo/v4"
	"net/http"
	"os/exec"
)

type BridgeController struct {
	db            *gorm.DB
	activeBridges sync.Map
}

func NewBridgeController(db *gorm.DB) *BridgeController {
	return &BridgeController{
		db: db,
	}
}

type StartBridgeRequest struct {
	TransferRequestID int    `json:"transfer_request_id"`
	MountPath         string `json:"mount_path"`
	LogPath           string `json:"log_path"`
}

type ActiveBridge struct {
	TransferRequestID int    `json:"transfer_request_id"`
	MountPath         string `json:"mount_path"`
	Pid               int    `json:"pid"`
}

func (c *BridgeController) ListActiveBridgesController(ctx echo.Context) error {
	var resp []ActiveBridge

	c.activeBridges.Range(func(key, value interface{}) bool {
		runningMount := value.(ActiveBridge)
		resp = append(resp, runningMount)
		return true
	})

	return ctx.JSON(http.StatusOK, &resp)
}

func (c *BridgeController) StopBridgeController(ctx echo.Context) error {
	var req struct {
		TransferRequestID int `json:"transfer_request_id"`
	}

	if err := ctx.Bind(&req); err != nil {
		return err
	}

	transferRequest := mcmodel.TransferRequest{ID: req.TransferRequestID}

	err := c.db.Model(&transferRequest).Update("state", "closed").Error
	if err != nil {
		return err
	}

	return ctx.NoContent(http.StatusOK)
}

func (c *BridgeController) StartBridgeController(ctx echo.Context) error {
	var req StartBridgeRequest

	if err := ctx.Bind(&req); err != nil {
		return err
	}

	// Run in background
	go c.startBridge(req)

	return ctx.NoContent(http.StatusOK)
}

func (c *BridgeController) startBridge(req StartBridgeRequest) {

	cmd := exec.Command("nohup", "/usr/local/bin/mcbridgefs.sh", fmt.Sprintf("%d", req.TransferRequestID),
		req.MountPath, req.LogPath)
	if err := cmd.Start(); err != nil {
		log.Errorf("Starting bridge failed (%d, %s): %s", req.TransferRequestID, req.MountPath, err)
		return
	}

	activeBridge := ActiveBridge{
		TransferRequestID: req.TransferRequestID,
		MountPath:         req.MountPath,
		Pid:               cmd.Process.Pid,
	}

	// Store running bridge so it can be queried and tracked
	c.activeBridges.Store(req.MountPath, activeBridge)

	if err := cmd.Wait(); err != nil {
		log.Errorf("Bridge exited with error: %s", err)
	}

	c.activeBridges.Delete(req.MountPath)
}
