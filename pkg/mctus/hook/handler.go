package hook

import (
	"fmt"
	"sync"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/tus/tusd/v2/pkg/hooks"
	"gorm.io/gorm"
)

const ClearCacheCount = 10000

type MCHookHandler struct {
	projectStor stor.ProjectStor
	fileStor    stor.FileStor
	userStor    stor.UserStor
	//projectCacheByID    map[int]*mcmodel.Project
	dirCacheByID        map[int]*mcmodel.File
	userCacheByAPIKey   map[string]*mcmodel.User
	userIDToProjectList map[int][]int
	accessCount         int
	mu                  sync.Mutex
}

func NewMCHookHandler(db *gorm.DB) *MCHookHandler {
	return &MCHookHandler{
		projectStor:         stor.NewGormProjectStor(db),
		fileStor:            stor.NewGormFileStor(db, ""),
		userStor:            stor.NewGormUserStor(db),
		dirCacheByID:        make(map[int]*mcmodel.File),
		userCacheByAPIKey:   make(map[string]*mcmodel.User),
		userIDToProjectList: make(map[int][]int),
		accessCount:         0,
	}
}

func (h *MCHookHandler) Setup() error {
	return nil
}

func (h *MCHookHandler) InvokeHook(req hooks.HookRequest) (res hooks.HookResponse, err error) {
	if req.Type != hooks.HookPreCreate {
		// This handler is only for hooks.HookPreCreate. If it's anything else
		// just return success so the request can continue.
		return res, nil
	}

	//
	if h.accessCount <= ClearCacheCount {
		h.accessCount = 0
		clear(h.dirCacheByID)
		clear(h.userCacheByAPIKey)
		clear(h.userIDToProjectList)
	}

	h.accessCount++

	reqMetaData, err := loadRequiredMetaData(req.Event.Upload.MetaData)
	if err != nil {
		return rejectRequest(err.Error()), nil
	}

	if err := h.validateRequest(reqMetaData); err != nil {
		return rejectRequest(err.Error()), nil
	}

	// Return empty res, this will signify it passed
	return res, nil
}

func (h *MCHookHandler) validateRequest(reqMetaData *requiredMetaData) error {
	// Validate the API Token by finding the associated user.
	user, err := h.getUserByAPIToken(reqMetaData.apiToken)
	if err != nil {
		return err
	}

	// Now that we can an integer project id and a user (from the api_token) check
	// if the user has access to the project.
	if !h.userCanAccessProject(user.ID, reqMetaData.projectID) {
		return fmt.Errorf("no such project: %d", reqMetaData.projectID)
	}

	// Check that the given directory_id exists.
	dir, err := h.getFileByID(reqMetaData.directoryID)
	if err != nil {
		return fmt.Errorf("no such directory_id: %d", reqMetaData.directoryID)
	}

	// Make sure that id actually identifies a directory
	if dir.IsDir() {
		return fmt.Errorf("id %d is not a directory", reqMetaData.directoryID)
	}

	// Ensure that the given directory_id exists in the identified project.
	if dir.ProjectID != reqMetaData.projectID {
		return fmt.Errorf("invalid directory_id %d for project %d", dir.ID, reqMetaData.projectID)
	}

	return nil
}

func (h *MCHookHandler) getUserByAPIToken(apiToken string) (*mcmodel.User, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if val, ok := h.userCacheByAPIKey[apiToken]; ok {
		return val, nil
	}

	user, err := h.userStor.GetUserByAPIToken(apiToken)
	if err != nil {
		return nil, err
	}

	h.userCacheByAPIKey[apiToken] = user
	return user, nil
}

func (h *MCHookHandler) userCanAccessProject(userID int, projectID int) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if projList, ok := h.userIDToProjectList[userID]; ok {
		for _, projID := range projList {
			if projID == projectID {
				return true
			}
		}

		if h.projectStor.UserCanAccessProject(userID, projectID) {
			h.userIDToProjectList[userID] = append(h.userIDToProjectList[userID], projectID)
			return true
		}
	}

	return false
}

func (h *MCHookHandler) getFileByID(id int) (*mcmodel.File, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if val, ok := h.dirCacheByID[id]; ok {
		return val, nil
	}

	dir, err := h.fileStor.GetFileByID(id)
	if err != nil {
		return nil, err
	}

	h.dirCacheByID[id] = dir
	return dir, nil
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
