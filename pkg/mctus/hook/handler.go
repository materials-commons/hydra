package hook

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	tusdh "github.com/tus/tusd/v2/pkg/handler"
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
		return rejectRequest(err), nil
	}

	if err := h.validateRequest(reqMetaData); err != nil {
		return rejectRequest(err), nil
	}

	// Return empty res, this will signify it passed
	return res, nil
}

type requiredMetaData struct {
	relativePath string
	apiToken     string
	projectID    int
	directoryID  int
}

func loadRequiredMetaData(metaData tusdh.MetaData) (*requiredMetaData, error) {
	var (
		reqMetaData requiredMetaData
		ok          bool
		err         error
	)

	// Make sure there is a relativePath passed in as this is needed to determine the directory.
	if reqMetaData.relativePath, ok = getMetaDataKey(metaData, "relativePath"); !ok {
		return nil, fmt.Errorf("no relativePath field")
	}

	// An api_token must be passed in; this allows us to authenticate the request.
	if reqMetaData.apiToken, ok = getMetaDataKey(metaData, "api_token"); !ok {
		return nil, fmt.Errorf("no api_token field")
	}

	// A request is associated with a project_id, make sure one was passed.
	projectID, ok := getMetaDataKey(metaData, "project_id")
	if !ok {
		return nil, fmt.Errorf("no project_id field")
	}

	// ProjectIDs are integers. Make sure an integer was passed in.
	reqMetaData.projectID, err = strconv.Atoi(projectID)
	if err != nil {
		return nil, fmt.Errorf("project_id field must be an integer")
	}

	// At a top level the directory_id identifies the base directory the files
	// are being uploaded into.This is different from relativePath. The relativePath
	// determines any subdirectories in the base directory identified by directory_id.
	directoryID, ok := getMetaDataKey(metaData, "directory_id")
	if !ok {
		return nil, fmt.Errorf("no directory_id field")
	}

	// directory_id must be an integer.
	reqMetaData.directoryID, err = strconv.Atoi(directoryID)
	if err != nil {
		return nil, fmt.Errorf("directory_id field must be an integer")
	}

	return &reqMetaData, nil
}

func getMetaDataKey(metaData tusdh.MetaData, key string) (string, bool) {
	val, ok := metaData[key]
	return val, ok
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

func rejectRequest(reason error) (res hooks.HookResponse) {
	res.RejectUpload = true
	res.HTTPResponse.StatusCode = 400
	res.HTTPResponse.Body = reason.Error()
	return res
}
