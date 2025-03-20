package webapi

import (
	"errors"
	"github.com/apex/log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcapid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcssh/mc"
)

type TransferUploadController struct {
	clientTransferStor      stor.ClientTransferStor
	fileStor                stor.FileStor
	transferRequestStor     stor.TransferRequestStor
	transferRequestFileStor stor.TransferRequestFileStor
	clientTransferCache     mcapid.ClientTransferCache
}

type TransferUploadControllerConfig struct {
	ClientTransferStor      stor.ClientTransferStor
	FileStor                stor.FileStor
	TransferRequestStor     stor.TransferRequestStor
	TransferRequestFileStor stor.TransferRequestFileStor
	ClientTransferCache     mcapid.ClientTransferCache
}

func NewTransferUploadController(cfg TransferUploadControllerConfig) *TransferUploadController {
	return &TransferUploadController{
		clientTransferStor:      cfg.ClientTransferStor,
		fileStor:                cfg.FileStor,
		transferRequestStor:     cfg.TransferRequestStor,
		transferRequestFileStor: cfg.TransferRequestFileStor,
		clientTransferCache:     cfg.ClientTransferCache,
	}
}

type StartUploadRequest struct {
	DestinationPath  string `json:"destination_path"`
	ProjectID        int    `json:"project_id"`
	ExpectedSize     int64  `json:"expected_size"`
	ClientUUID       string `json:"client_uuid"`
	ExpectedChecksum string `json:"expected_checksum"`
	ClientModTime    string `json:"client_mod_time"`
}

func (c *TransferUploadController) StartUpload(ctx echo.Context) error {
	var req StartUploadRequest

	if err := ctx.Bind(&req); err != nil {
		return ctx.NoContent(http.StatusBadRequest)
	}

	user := ctx.Get("user").(*mcmodel.User)
	transferRequestFile, err := c.clientTransferCache.GetTransferRequestFileByPath(req.ClientUUID, req.ProjectID, user.ID, req.DestinationPath)
	switch {
	case errors.Is(err, mcapid.ErrNoClientTransfer):
		// Need to create a ClientTransfer, a TransferRequest, and a TransferRequestFile
		transferRequestFile, err = c.clientTransferCache.WithWriteLockHeld(func(cache mcapid.NoLockHeldClientTransferCache) (*mcmodel.TransferRequestFile, error) {
			trf, err := cache.GetTransferRequestFileByPathNoLockHeld(req.ClientUUID, req.ProjectID, user.ID, req.DestinationPath)
			if err == nil {
				// Someone slipped in while we were acquiring the write lock and created everything
				return trf, nil
			}

			return c.createClientTransferAndTransferRequestFile(cache, req, user.ID)
		})

		if err != nil {
			return err
		}

		// return json blob with success
	case errors.Is(err, mcapid.ErrNoMatchingClientTransferRequestFile):
		transferRequestFile, err = c.clientTransferCache.WithWriteLockHeld(func(cache mcapid.NoLockHeldClientTransferCache) (*mcmodel.TransferRequestFile, error) {
			trf, err := cache.GetTransferRequestFileByPathNoLockHeld(req.ClientUUID, req.ProjectID, user.ID, req.DestinationPath)
			if err == nil {
				return trf, nil
			}

			transferRequest, err := cache.GetTransferRequestNoLockHeld(req.ClientUUID, req.ProjectID, user.ID)
			if err != nil {
				return nil, err
			}

			trf, err = c.createTransferRequestFile(transferRequest, req, user.ID)

			if err = cache.InsertTransferRequestFileNoLockHeld(req.ClientUUID, trf); err != nil {
				// If this happens something really went wrong as we already know that the ClientUUID exists
				// and yet now we've failed while holding the lock that would prevent anyone else from changing
				// the cache!!
				log.Debugf("Failed to insert transferRequestFile with ID (%d) into cache for ClientUUID %s. This should never happen.", trf.ID, req.ClientUUID)
				return nil, err
			}

			return trf, nil
		})
		// Need to create a TransferRequestFile
	case err != nil:
	// some other error
	default:
		// We have a transferRequestFile. We need to check the upload state and send back the offset to start
		// sending bytes. Before we do this, we need to check if the file completed uploading, by checking
		// the expected size against the current file size.
		f, err := c.fileStor.GetFileByID(transferRequestFile.FileID)
		if err != nil {
			// how to handle this case... This is a fatal issue...
			return err
		}

		finfo, err := os.Stat(f.ToUnderlyingFilePath(c.fileStor.Root()))
		if err != nil {
		}

		if finfo.Size() == req.ExpectedSize {

		}
	}

	return nil
}

func (c *TransferUploadController) createClientTransferAndTransferRequestFile(cache mcapid.NoLockHeldClientTransferCache, req StartUploadRequest, ownerID int) (*mcmodel.TransferRequestFile, error) {
	// Create the TransferRequest
	transferRequest := &mcmodel.TransferRequest{
		ProjectID: req.ProjectID,
		OwnerID:   ownerID,
		State:     "open",
	}

	tr, err := c.transferRequestStor.CreateTransferRequest(transferRequest)
	if err != nil {
		return nil, err
	}

	trf, err := c.createTransferRequestFile(tr, req, ownerID)
	if err != nil {
		return nil, err
	}

	ct := &mcmodel.ClientTransfer{
		ClientUUID:        req.ClientUUID,
		ProjectID:         req.ProjectID,
		OwnerID:           ownerID,
		TransferRequestID: tr.ID,
	}

	ct, err = c.clientTransferStor.CreateClientTransfer(ct)
	if err != nil {
		return nil, err
	}

	cache.InsertClientTransferNoLockHeld(req.ClientUUID, ct)
	if err := cache.InsertTransferRequestFileNoLockHeld(req.ClientUUID, trf); err != nil {
		return nil, err
	}

	return trf, nil
}

func (c *TransferUploadController) createTransferRequestFile(tr *mcmodel.TransferRequest, req StartUploadRequest, userID int) (*mcmodel.TransferRequestFile, error) {
	dir, err := c.fileStor.GetOrCreateDirPath(req.ProjectID, userID, filepath.Dir(req.DestinationPath))
	if err != nil {
		return nil, err
	}

	filename := filepath.Base(req.DestinationPath)
	f, err := c.fileStor.CreateFile(filename, req.ProjectID, userID, dir.ID, mc.GetMimeType(filename))
	if err != nil {
		return nil, err
	}

	_, trf, err := c.transferRequestStor.CreateNewFile(f, dir, tr)

	return trf, err
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
