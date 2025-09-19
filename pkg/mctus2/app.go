package mctus2

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcssh/mc"
	tusd "github.com/tus/tusd/v2/pkg/handler"
	"gorm.io/gorm"
)

type App struct {
	TusFileStore   LocalFileStore
	TusHandler     *tusd.Handler
	projectStor    stor.ProjectStor
	fileStor       stor.FileStor
	userStor       stor.UserStor
	conversionStor stor.ConversionStor
	accessCache    *AccessCache
	accessCount    int
	mcfsDir        string
}

func NewApp(tusFileStore LocalFileStore, tusHandler *tusd.Handler, db *gorm.DB, mcfsDir string) *App {
	return &App{
		TusFileStore:   tusFileStore,
		TusHandler:     tusHandler,
		projectStor:    stor.NewGormProjectStor(db),
		fileStor:       stor.NewGormFileStor(db, mcfsDir),
		userStor:       stor.NewGormUserStor(db),
		conversionStor: stor.NewGormConversionStor(db),
		accessCache:    NewAccessCache(),
		mcfsDir:        mcfsDir,
	}
}

func (a *App) getUserByAPIToken(apiToken string) (*mcmodel.User, error) {
	a.accessCache.Lock()
	defer a.accessCache.Unlock()

	if val, ok := a.accessCache.usersByAPIKey[apiToken]; ok {
		return val, nil
	}

	// user was not found in cache so look up in db
	user, err := a.userStor.GetUserByAPIToken(apiToken)
	if err != nil {
		return nil, err
	}

	a.accessCache.usersByAPIKey[apiToken] = user
	return user, nil
}

func (a *App) userCanAccessProject(userID int, projectID int) bool {
	a.accessCache.Lock()
	defer a.accessCache.Unlock()

	if projList, ok := a.accessCache.userIDToProjectList[userID]; ok {
		for _, projID := range projList {
			if projID == projectID {
				return true
			}
		}

		if a.projectStor.UserCanAccessProject(userID, projectID) {
			a.accessCache.userIDToProjectList[userID] = append(a.accessCache.userIDToProjectList[userID], projectID)
			return true
		}
	}

	return false
}

func (a *App) AccessMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isCreate := r.Method == http.MethodPost && path.Clean(r.URL.Path) == "/"
		if !isCreate {
			next.ServeHTTP(w, r)
			return
		}

		metadata := parseTusMetadata(r.Header.Get("Upload-Metadata"))
		projectID, err := strconv.Atoi(metadata["project_id"])
		if err != nil {
			http.Error(w, "project_id not found", http.StatusBadRequest)
			return
		}

		directoryPath := metadata["directory_path"]
		if directoryPath == "" {
			http.Error(w, "directory_path not found", http.StatusBadRequest)
			return
		}

		filename := metadata["filename"]
		if filename == "" {
			http.Error(w, "filename not found", http.StatusBadRequest)
			return
		}

		userID, err := strconv.Atoi(metadata["user_id"])
		if err != nil {
			http.Error(w, "user_id not found", http.StatusBadRequest)
			return
		}

		apiToken, err := apiTokenFromRequest(r)

		user, err := a.getUserByAPIToken(apiToken)
		if err != nil {
			http.Error(w, "invalid api token", http.StatusUnauthorized)
			return
		}

		if user.ID != userID {
			http.Error(w, "invalid user id", http.StatusUnauthorized)
		}

		if !a.userCanAccessProject(user.ID, projectID) {
			http.Error(w, "no such project", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (a *App) OnFileComplete() {
	for event := range a.TusHandler.CompleteUploads {
		uploadedFileInfo := event.Upload
		metadata := uploadedFileInfo.MetaData
		projectID, err := strconv.Atoi(metadata["project_id"])
		if err != nil {
			continue
		}

		userID, err := strconv.Atoi(metadata["user_id"])
		if err != nil {
			continue
		}

		directoryPath := metadata["directory_path"]
		filename := metadata["filename"]
		if directoryPath == "" || filename == "" {
			continue
		}

		// We are going to move the file from TUS storage into Materials Commons.
		// This will require us to create a new file entry in the database, and
		// possibly a set of directories. Then we will need to move the file from
		// TUS storage to Materials Commons file storage.

		// Step 1: Get the directory entry for the directory path. The call
		//         will create the directory path if needed

		dir, err := a.fileStor.GetOrCreateDirPath(projectID, userID, directoryPath)
		if err != nil {
			continue
		}

		// Step 2: Create a new file entry in the database. This entry
		//         will not be complete. After we move the file, we can
		//         update missing information in this file entry.

		f, err := a.fileStor.CreateFile(filename, projectID, dir.ID, userID, mc.GetMimeType(filename))

		// Step 3: Move the file from TUS storage to Materials Commons file storage.
		//         Along the way we can also compute the checksum, and get the file size.

		// Lets get the path to the file in TUS storage

		tusFilePath := a.TusFileStore.GetFilePath(uploadedFileInfo.ID)

		// Make sure the directory for the file exists
		if err := f.MkdirUnderlyingPath(a.mcfsDir); err != nil {
			// do something
		}

		// Move the file from TUS storage to Materials Commons file storage
		if err := os.Rename(tusFilePath, f.ToUnderlyingFilePath(a.mcfsDir)); err != nil {
			// do something
		}

		// Step 4: Update the file entry in the database with the checksum and size.
		//         This will allow us to serve the file to the user.

		hasher := md5.New()
		fh, err := os.Open(f.ToUnderlyingFilePath(a.mcfsDir))
		if _, err := io.Copy(hasher, fh); err != nil {
			// do something
		}
		sum := hasher.Sum(nil)
		checksum := fmt.Sprintf("%x", sum)

		finfo, err := fh.Stat()
		_, _ = a.fileStor.DoneWritingToFile(f, checksum, finfo.Size(), a.conversionStor)

		// Step 5: Remove the info file from TUS storage.
		//         This is not strictly necessary, but it is good practice.
		//         We can always recover the info file from the database.

		ctx := context.Background()
		up, err := a.TusFileStore.GetUpload(ctx, uploadedFileInfo.ID)
		if t, ok := up.(tusd.TerminatableUpload); ok {
			_ = t.Terminate(ctx)
		}
	}
}

func parseTusMetadata(metadata string) map[string]string {
	metadataMap := make(map[string]string)
	for _, kv := range strings.Split(metadata, ",") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}

		// kvParts[0] = key, kvParts[1] = value
		kvParts := strings.Split(kv, " ")
		if len(kvParts) != 2 {
			continue
		}
		key := kvParts[0]
		val := kvParts[1]
		value, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			continue
		}
		metadataMap[key] = string(value)
	}
	return metadataMap
}

func apiTokenFromRequest(r *http.Request) (string, error) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return "", errors.New("authorization header not found")
	}

	parts := strings.Fields(auth)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("authorization header is not a bearer token")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("bearer token is empty")
	}

	return token, nil
}
