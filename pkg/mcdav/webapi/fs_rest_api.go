package webapi

import (
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcdav"
	"github.com/materials-commons/hydra/pkg/mcdav/fs"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type FSRestAPIHandler struct {
	userStor stor.UserStor
	users    *mcdav.Users
}

func NewFSRestAPI(userStor stor.UserStor, users *mcdav.Users) *FSRestAPIHandler {
	return &FSRestAPIHandler{
		userStor: userStor,
		users:    users,
	}
}

func (h *FSRestAPIHandler) ResetUserStateHandler(c echo.Context) error {
	var req struct {
		Email string `json:"email"`
	}

	if err := c.Bind(&req); err != nil {
		return err
	}

	_, err := h.userStor.GetUserByEmail(req.Email)
	if err != nil {
		// no such user...
		return err
	}

	if userEntry := h.users.GetUserByUsername(req.Email); userEntry != nil {
		userFS := userEntry.Server.FileSystem.(*fs.UserFS)
		userFS.ResetState()
	}

	return nil
}
