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
	"sync"

	"github.com/apex/log"
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
	ctx            context.Context
	maxParallel    int
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
		maxParallel:    5,
		ctx:            context.Background(),
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

type FileMetadata struct {
	ProjectID     int
	UserID        int
	DirectoryPath string
	Filename      string
}

func (a *App) getUploadedFileMetadata(metadata tusd.MetaData) (*FileMetadata, error) {
	var (
		fileMetadata FileMetadata
		err          error
	)

	fileMetadata.ProjectID, err = strconv.Atoi(metadata["project_id"])
	if err != nil {
		return nil, err
	}

	fileMetadata.UserID, err = strconv.Atoi(metadata["user_id"])
	if err != nil {
		return nil, err
	}

	fileMetadata.DirectoryPath = metadata["directory_path"]
	fileMetadata.Filename = metadata["filename"]
	if fileMetadata.DirectoryPath == "" || fileMetadata.Filename == "" {
		return nil, err
	}

	return &fileMetadata, nil
}

func (a *App) OnFileComplete() {
	uploadCompleteEvents := a.TusHandler.CompleteUploads
	var wg sync.WaitGroup

	// Create the background worker for the file completes
	fileCompleteWorker := func() {
		defer wg.Done()
		for {
			select {
			case <-a.ctx.Done():
				return
			case ev, ok := <-uploadCompleteEvents:
				if !ok {
					return
				}
				if err := a.handleFileComplete(ev); err != nil {
					log.Errorf("Error handling file complete: %v", err)
				}
			}
		}
	}

	// Set the parallelism
	wg.Add(a.maxParallel)

	// Launch the background workers that will process completion events
	for i := 0; i < a.maxParallel; i++ {
		go fileCompleteWorker()
	}

	// Wait forever, or until the background workers receive a close event and exit.
	wg.Wait()
}

func (a *App) handleFileComplete(event tusd.HookEvent) error {
	uploadedFileInfo := event.Upload
	metadata, err := a.getUploadedFileMetadata(uploadedFileInfo.MetaData)
	if err != nil {
		// log error and skip this upload
		return err
	}

	tusFilePath := a.TusFileStore.GetFilePath(uploadedFileInfo.ID)
	checksum, err := computeChecksum(tusFilePath)
	if err != nil {
		return err
	}

	dir, err := a.fileStor.GetOrCreateDirPath(metadata.ProjectID, metadata.UserID, metadata.DirectoryPath)
	if err != nil {
		return err
	}

	//fmt.Printf("uploadedFileInfo.ID: %s\n", uploadedFileInfo.ID)
	existingFile, err := a.fileStor.GetMatchingFileInDirectory(dir.ID, checksum, metadata.Filename)
	switch {
	case err != nil:
		// Assume we need to create a new file
		return a.handleUploadOfNewFile(metadata, checksum, dir, uploadedFileInfo)
	case existingFile == nil:
		// No matching file found, so we need to create a new file
		return a.handleUploadOfNewFile(metadata, checksum, dir, uploadedFileInfo)
	default:
		// The existingFile is not null, so handle upload of an existing entry
		return a.handleUploadOfExistingFile(metadata, checksum, dir, existingFile, uploadedFileInfo)
	}
}

func computeChecksum(filePath string) (string, error) {
	hasher := md5.New()
	fh, err := os.Open(filePath)
	if err != nil {
		return "", err
	}

	defer fh.Close()

	if _, err := io.Copy(hasher, fh); err != nil {
		return "", err
	}
	sum := hasher.Sum(nil)
	return fmt.Sprintf("%x", sum), nil
}

func (a *App) handleUploadOfNewFile(metadata *FileMetadata, checksum string, dir *mcmodel.File, info tusd.FileInfo) error {
	//fmt.Printf("handleUploadOfNewFile: %s\n", info.ID)
	usesUuid := ""

	tusFilePath := a.TusFileStore.GetFilePath(info.ID)
	mimeType := mc.DetectMimeType(tusFilePath)

	// Check if there is already a file with the same checksum anywhere in the system.
	matchingFileByChecksum, _ := a.fileStor.FindMatchingFileByChecksum(checksum)

	// Ignore the error. We assume if matchingFileByChecksum is nil then there is no match. The worst case
	// is we have a second copy of a file.

	// Create the new file entry in the database.
	f, err := a.fileStor.CreateFile(metadata.Filename, metadata.ProjectID, dir.ID, metadata.UserID, mimeType)
	if err != nil {
		return err
	}

	// A review of the logic for this part of the code. There are 3 cases:
	//
	// 1. We did not find a matching file with the same checksum. In that case we need to save the file
	//    to disk using the UUID for the fileEntry we just created.
	// 2. We found a matching file with the same checksum, but that file does not exist on disk. In that
	//    case we need to save the file to disk to the appropriate UUID for the matchingFileByChecksum
	//    entry and mark the matching file as fixed.
	// 3. We found a matching file with the same checksum, and that file exists on disk. In that case
	//    we don't need to save anything to disk, because we already have a file with the same checksum.

	if matchingFileByChecksum == nil {
		// If there is not a file with a matching checksum, then this is a completely
		// new file. We save the file to the generated UUID.
		//
		// This is case (1) above.
		if err := a.moveFileTo(info, f.ToUnderlyingFilePath(a.mcfsDir)); err != nil {
			// Review if this is the right thing to do.
			return err
		}
	} else {
		// There was a matching file found, so we want to set this file's uses_uuid to point at the matching
		// file. At this point the match is either the original file that everything points at or it's a file
		// containing the pointer to the original file (the pointer is uses_uuid).
		usesUuid = matchingFileByChecksum.UUIDForUses()

		if !matchingFileByChecksum.RealFileExists(a.mcfsDir) {
			// The matching file does not exist on disk, so save it and mark it as fixed.
			// This is case (2) above
			//
			// There is nothing we need to do with the fileEntry here, because its health is already set to 'good'.
			// We only need to set the health of the matching file to 'fixed'.
			if err := a.moveFileTo(info, matchingFileByChecksum.ToUnderlyingFilePath(a.mcfsDir)); err != nil {
				// log err
				log.Errorf("failed moving file to expected location: %s", err)
			} else {
				_, _ = a.fileStor.SetFileHealthFixed(matchingFileByChecksum, "tus-upload:existence-check", "TUS")
			}
		}
		// else if matchingFileByChecksum->realFileExists() is true, then we don't need to do anything.
		// This is case (3) above.
	}

	// At this point the file exists on disk. Let's update the database entry and refresh our state.
	// We need to update the size, uses_uuid if not blank, and then appropriately set the current flag.
	updates := mcmodel.File{
		Size:         uint64(info.Size),
		UploadSource: "TUS",
		UsesUUID:     usesUuid,
	}

	f, err = a.fileStor.UpdateFile(f, &updates)
	if _, err := a.fileStor.SetFileAsCurrent(f); err != nil {
		log.Errorf("failed setting file %d as current: %s", f.ID, err)
	}

	if _, err := a.conversionStor.AddFileToConvert(f); err != nil {
		log.Errorf("failed adding file %d to be converted: %s", f.ID, err)
	}

	if !f.RealFileExists(a.mcfsDir) {
		_, _ = a.fileStor.SetFileHealthMissing(f, "tus-upload:existence-check", "TUS")
		log.Errorf("file was not moved to expected location")
	}

	a.cleanupTUSFiles(info)

	return nil
}

func (a *App) handleUploadOfExistingFile(metadata *FileMetadata, checksum string, dir *mcmodel.File, existingFile *mcmodel.File, info tusd.FileInfo) error {
	// There are two cases here we need to account for:
	// 1. The existingFile file is not on disk. In this case we need to mark it as fixed and save it.
	// 2. The existingFile is on disk, in that case we don't need to save anything to disk.
	//fmt.Printf("handleUploadOfExistingFile: %s\n", info.ID)

	if !existingFile.RealFileExists(a.mcfsDir) {
		_, _ = a.fileStor.SetFileHealthFixed(existingFile, "tus-upload:existence-check", "TUS")
		pathToMoveFileTo := existingFile.ToUnderlyingFilePath(a.mcfsDir)
		if err := a.moveFileTo(info, pathToMoveFileTo); err != nil {
			return err
		}

		if !existingFile.RealFileExists(a.mcfsDir) {
			_, _ = a.fileStor.SetFileHealthMissing(existingFile, "tus-upload:existence-check", "TUS")
			return errors.New("file was not moved to expected location")
		}
	}

	// The existingFile exists on disk. This is case 2. Add the existingFile to the conversion queue
	// and set the current flag on the existingFile, and any files matching its name in the directory.
	if _, err := a.conversionStor.AddFileToConvert(existingFile); err != nil {
		log.Errorf("failed adding file %d to be converted: %s", existingFile.ID, err)
	}

	if _, err := a.fileStor.SetFileAsCurrent(existingFile); err != nil {
		log.Errorf("failed setting file %d as current: %s", existingFile.ID, err)
	}

	a.cleanupTUSFiles(info)

	return nil
}

func (a *App) moveFileTo(info tusd.FileInfo, to string) error {
	tusFilePath := a.TusFileStore.GetFilePath(info.ID)
	//fmt.Printf("moveFileTo: %s to %s\n", tusFilePath, to)
	if err := os.MkdirAll(path.Dir(to), 0755); err != nil {
		log.Errorf("failed creating directory for path %s: %s", path.Dir(to), err)
	}
	if err := os.Rename(tusFilePath, to); err != nil {
		log.Errorf("failed moving file %s to expected location %s: %s", tusFilePath, to, err)
		return err
	}

	return nil
}

// cleanupTUSFile delete the TUS upload info file, and the upload file itself. The
// upload file may not exist if the upload was moved. The upload file is moved
// if there isn't a file with the same checksum already stored in the system.
func (a *App) cleanupTUSFiles(info tusd.FileInfo) {
	infoPath := a.TusFileStore.GetInfoPath(info.ID)
	if err := os.Remove(infoPath); err != nil {
		log.Errorf("failed deleting TUS upload info (%s): %s", infoPath, err)
	}

	// Ignore error, as the file may not exist.
	_ = os.Remove(a.TusFileStore.GetFilePath(info.ID))
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
