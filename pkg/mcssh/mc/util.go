package mc

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

// RemoveProjectSlugFromPath removes the slug project name from the path. For example the slug
// project name might be my-project-acf4. All paths will be prefixed with /my-project-acf4. So if
// the path is /my-project-acf4/file.txt then this method will return /file.txt
func RemoveProjectSlugFromPath(path, projectSlug string) string {
	cleanedPath := filepath.Clean(path)
	sluggedNamePath := filepath.Join("/", projectSlug)

	if strings.HasPrefix(cleanedPath, sluggedNamePath) {
		cleanedPath = strings.TrimPrefix(cleanedPath, sluggedNamePath)
	}

	if cleanedPath == "" {
		return "/"
	}

	return cleanedPath
}

// GetProjectSlugFromPath extracts the project slug from the beginning of the path. For example /my-project/this/that
// has a project slug of "my-project".
func GetProjectSlugFromPath(path string) string {
	parts := strings.Split(path, "/")
	// parts will be an array with the first element being an
	// empty string. For example if the path is /my-project/this/that,
	// then the array will be:
	// ["", "my-project", "this", "that"]
	// So the project slug is parts[1]
	projectSlug := parts[1]

	return projectSlug
}

// GetMimeTypeByExtension will determine the type of file from its extension. It strips out the extra information
// such as the charset and just returns the underlying type. It returns "unknown" for the mime type if
// the mime package is unable to determine the type.
func GetMimeTypeByExtension(name string) string {
	mimeType := mime.TypeByExtension(filepath.Ext(name))
	if mimeType == "" {
		return "unknown"
	}

	semicolon := strings.Index(mimeType, ";")
	if semicolon == -1 {
		return strings.TrimSpace(mimeType)
	}

	return strings.TrimSpace(mimeType[:semicolon])
}

func DetectMimeType(filePath string) string {
	// Attempt to determine the mime type from file contents
	buf := make([]byte, 512)
	f, _ := os.Open(filePath)
	defer f.Close()
	n, _ := f.Read(buf)
	mimeType := http.DetectContentType(buf[:n])

	// Handle the case where the mime type has extra information after a semicolon,
	// for example, text/plain; charset=utf-8
	semicolon := strings.Index(mimeType, ";")
	if semicolon == -1 {
		mimeType = strings.TrimSpace(mimeType)
	} else {
		mimeType = strings.TrimSpace(mimeType[:semicolon])
	}

	switch {
	case mimeType == "":
		return GetMimeTypeByExtension(filePath)
	case mimeType == "application/octet-stream" || mimeType == "text/plain":
		// If the mime type is application/octet-stream or text/plain,
		// then try to determine the mime type from the extension.
		saveMimeType := mimeType
		mimeType = GetMimeTypeByExtension(filePath)

		// If we can't determine the mime type from the extension, then return the saved mime type.
		if mimeType == "unknown" {
			return saveMimeType
		}
		return mimeType
	default:
		return mimeType
	}
}

// GetAndValidateProjectFromPath retrieves the project by extracting the project slug from the path, and asking
// the project store for that project. It also validates that the userID passed in has access to
// the project.
func GetAndValidateProjectFromPath(path string, userID int, projectStore stor.ProjectStor) (*mcmodel.Project, error) {
	// Look up the project by the slug in the path. Each path needs to have the project slug encoded in it
	// so that we know which project the user is accessing.
	projectSlug := GetProjectSlugFromPath(path)

	project, err := projectStore.GetProjectBySlug(projectSlug)
	if err != nil {
		log.Errorf("No such project slug %s", projectSlug)
		return nil, err
	}

	// Once we have the project we need to check that the user has access to the project.
	if !projectStore.UserCanAccessProject(userID, project.ID) {
		log.Errorf("User %d doesn't have access to project %d (%s)", userID, project.ID, project.Slug)
		return nil, fmt.Errorf("no such project %s", projectSlug)
	}

	return project, nil
}
