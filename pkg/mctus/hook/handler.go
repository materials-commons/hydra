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
	if req.Type != hooks.HookPreCreate {
		// We only handle hooks.HookPreCreate
		return res, nil
	}

	// HookPreCreate handled from here on

	// Make sure there is a relativePath passed in as this is needed to determine the directory.
	if _, ok := reqHasMetadata(req, "relativePath"); !ok {
		return rejectRequest("no relativePath provided"), nil
	}

	// An api_token must be passed in; this allows us to authenticate the request.
	apiToken, ok := reqHasMetadata(req, "api_token")
	if !ok {
		return rejectRequest("no api_token provided"), nil
	}

	// Validate the API Token by finding the associated user.
	user, err := h.userStor.GetUserByAPIToken(apiToken)
	if err != nil {
		return rejectRequest("invalid api_token"), nil
	}

	// A request is associated with a project_id, make sure one was passed.
	projectID, ok := reqHasMetadata(req, "project_id")
	if !ok {
		return rejectRequest("no project_id provided"), nil
	}

	// ProjectIDs are integers. Make sure an integer was passed in.
	projID, err := strconv.Atoi(projectID)
	if err != nil {
		return rejectRequest("invalid project_id"), nil
	}

	// Now that we can an integer project id and a user (from the api_token) check
	// if the user has access to the project.
	if !h.projectStor.UserCanAccessProject(user.ID, projID) {
		return rejectRequest("no such project: " + projectID), nil
	}

	// At a top level the directory_id identifies the base directory the files
	// are being uploaded into.This is different from relativePath. The relativePath
	// determines any subdirectories in the base directory identified by directory_id.
	directoryID, ok := reqHasMetadata(req, "directory_id")
	if !ok {
		return rejectRequest("no directory_id provided"), nil
	}

	// directory_id must be an integer.
	dirID, err := strconv.Atoi(directoryID)
	if err != nil {
		return rejectRequest("invalid directory_id"), nil
	}

	// Check that the given directory_id exists.
	dir, err := h.fileStor.GetFileByID(dirID)
	if err != nil {
		return rejectRequest("no such directory_id"), nil
	}

	// Make sure that id actually identifies a directory
	if dir.IsDir() {
		return rejectRequest("id is not a directory"), nil
	}

	// Ensure that the given directory_id exists in the identified project.
	if dir.ProjectID != projID {
		return rejectRequest("invalid directory_id for project"), nil
	}

	// Return empty res, this will signify it passed
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
