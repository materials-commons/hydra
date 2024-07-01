package hook

import (
	"strconv"

	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/tus/tusd/v2/pkg/hooks"
	"gorm.io/gorm"
)

type MCHookHandler struct {
	projectStor stor.ProjectStor
	fileStor    stor.FileStor
	userStor    stor.UserStor
}

func NewMCHookHandler(db *gorm.DB) *MCHookHandler {
	return &MCHookHandler{
		projectStor: stor.NewGormProjectStor(db),
		fileStor:    stor.NewGormFileStor(db, ""),
		userStor:    stor.NewGormUserStor(db),
	}
}

func (h *MCHookHandler) Setup() error {
	return nil
}

func (h *MCHookHandler) InvokeHook(req hooks.HookRequest) (res hooks.HookResponse, err error) {
	if req.Type == hooks.HookPreCreate {
		if _, ok := reqHasMetadata(req, "relativePath"); !ok {
			return rejectRequest("no relativePath provided"), nil
		}

		apiToken, ok := reqHasMetadata(req, "api_token")
		if !ok {
			return rejectRequest("no api_token provided"), nil
		}

		user, err := h.userStor.GetUserByAPIToken(apiToken)
		if err != nil {
			return rejectRequest("invalid api_token"), nil
		}

		projectID, ok := reqHasMetadata(req, "project_id")
		if !ok {
			return rejectRequest("no project_id provided"), nil
		}

		projID, err := strconv.Atoi(projectID)
		if err != nil {
			return rejectRequest("invalid project_id"), nil
		}

		if !h.projectStor.UserCanAccessProject(user.ID, projID) {
			return rejectRequest("no such project: " + projectID), nil
		}

		directoryID, ok := reqHasMetadata(req, "directory_id")
		if !ok {
			return rejectRequest("no directory_id provided"), nil
		}

		dirID, err := strconv.Atoi(directoryID)
		if err != nil {
			return rejectRequest("invalid directory_id"), nil
		}

		dir, err := h.fileStor.GetFileByID(dirID)
		if err != nil {
			return rejectRequest("no such directory_id"), nil
		}

		if dir.ProjectID != projID {
			return rejectRequest("invalid directory_id for project"), nil
		}

		if dir.IsDir() {
			return rejectRequest("id is not a directory"), nil
		}

		// Return empty res, this will signify it passed
		return res, nil
	}
	return res, nil
}

func reqHasMetadata(req hooks.HookRequest, key string) (string, bool) {
	val, ok := req.Event.Upload.MetaData[key]
	return val, ok
}

func rejectRequest(reason string) (res hooks.HookResponse) {
	res.RejectUpload = true
	res.HTTPResponse.StatusCode = 400
	res.HTTPResponse.Body = reason
	return res
}
