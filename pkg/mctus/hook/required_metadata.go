package hook

import (
	"fmt"
	"strconv"

	tusdh "github.com/tus/tusd/v2/pkg/handler"
)

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
