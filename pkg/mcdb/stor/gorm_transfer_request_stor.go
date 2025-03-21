package stor

import (
	"os"
	"path/filepath"

	"github.com/apex/log"
	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/mime"
	"gorm.io/gorm"
)

type GormTransferRequestStor struct {
	db       *gorm.DB
	mcfsRoot string
}

func NewGormTransferRequestStor(db *gorm.DB, mcfsRoot string) *GormTransferRequestStor {
	return &GormTransferRequestStor{db: db, mcfsRoot: mcfsRoot}
}

// CreateTransferRequest creates a new TransferRequest, filling in the ID and UUID for the transfer request
// passed in.
func (s *GormTransferRequestStor) CreateTransferRequest(tr *mcmodel.TransferRequest) (*mcmodel.TransferRequest, error) {
	var err error

	if tr.UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	err = WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Create(tr).Error
	})
	if err != nil {
		return nil, err
	}

	return tr, nil
}

// ListTransferRequests returns the list of all active transfer requests. An active transfer
// request has its state set to "open".
func (s *GormTransferRequestStor) ListTransferRequests() ([]mcmodel.TransferRequest, error) {
	var transferRequests []mcmodel.TransferRequest

	err := s.db.Where("state = ?", "open").Find(&transferRequests).Error
	if err != nil {
		return nil, err
	}

	return transferRequests, nil
}

// MarkFileReleased should only be called for files that were created or opened with the Write flag set.
func (s *GormTransferRequestStor) MarkFileReleased(file *mcmodel.File, checksum string, projectID int, totalBytes int64) error {
	finfo, err := os.Stat(file.ToUnderlyingFilePath(s.mcfsRoot))
	if err != nil {
		log.Errorf("MarkFileReleased Stat %s failed: %s", file.ToUnderlyingFilePath(s.mcfsRoot), err)
		return err
	}

	return WithTxRetry(s.db, func(tx *gorm.DB) error {
		// To set file as the current (ie viewable) version we first need to set all its previous
		// versions to have current set to false.
		err := tx.Model(&mcmodel.File{}).
			Where("directory_id = ?", file.DirectoryID).
			Where("name = ?", file.Name).
			Update("current", false).Error

		if err != nil {
			return err
		}

		err = tx.Model(&mcmodel.TransferRequestFile{}).
			Where("file_id = ?", file.ID).
			Update("state", "closed").Error
		if err != nil {
			return err
		}

		// Now we can update the meta data on the current file. This includes, the size, current, and if there is
		// a new computed checksum, also update the checksum field.
		switch {
		case checksum != "":
			// If we are here then the file was written to so besides updating the file meta data we also have
			// to update the project size meta data
			fileMetadata := mcmodel.File{
				Size:     uint64(finfo.Size()),
				Current:  true,
				Checksum: checksum,
			}

			if err := tx.Model(file).Updates(&fileMetadata).Error; err != nil {
				return err
			}

			var project mcmodel.Project

			if result := tx.Find(&project, projectID); result.Error != nil {
				return result.Error
			}

			return tx.Model(&project).Updates(&mcmodel.Project{Size: project.Size + totalBytes}).Error
		default:
			// If we are here then the file was opened for read/write but it was never written to. In this situation there
			// is no checksum that has been computed, so don't update the field.
			return tx.Model(file).Updates(mcmodel.File{Size: uint64(finfo.Size()), Current: true}).Error
		}
	})
}

func (s *GormTransferRequestStor) MarkFileAsOpen(file *mcmodel.File) error {
	return WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Model(&mcmodel.TransferRequestFile{}).
			Where("file_id = ?", file.ID).
			Update("state", "open").Error
	})
}

func (s *GormTransferRequestStor) CreateNewFile(file, dir *mcmodel.File, transferRequest *mcmodel.TransferRequest) (*mcmodel.File, *mcmodel.TransferRequestFile, error) {
	var (
		err error
		trf *mcmodel.TransferRequestFile
	)
	if file, trf, err = s.addFileToDatabase(file, dir.ID, transferRequest, true); err != nil {
		return file, trf, err
	}

	if err := os.MkdirAll(file.ToUnderlyingDirPath(s.mcfsRoot), 0755); err != nil {
		// TODO: If this fails then we should remove the created file from the database
		log.Errorf("os.MkdirAll failed (%s): %s\n", file.ToUnderlyingDirPath(s.mcfsRoot), err)
		return nil, nil, err
	}

	file.Directory = dir
	return file, trf, nil
}

func (s *GormTransferRequestStor) CreateNewFileVersion(file, dir *mcmodel.File, transferRequest *mcmodel.TransferRequest) (*mcmodel.File, error) {
	var (
		err error
	)
	if file, _, err = s.addFileToDatabase(file, dir.ID, transferRequest, false); err != nil {
		return file, err
	}

	if err := os.MkdirAll(file.ToUnderlyingDirPath(s.mcfsRoot), 0755); err != nil {
		// TODO: If this fails then we should remove the created file from the database
		log.Errorf("os.MkdirAll failed (%s): %s\n", file.ToUnderlyingDirPath(s.mcfsRoot), err)
		return nil, err
	}

	file.Directory = dir
	return file, nil
}

// addFileToDatabase will add an mcmodel.File entry and an associated mcmodel.TransferRequestFile entry
// to the database. The file parameter must be filled out, except for the UUID which will be generated
// for the file. The TransferRequestFile will be created based on the file entry.
func (s *GormTransferRequestStor) addFileToDatabase(file *mcmodel.File, dirID int, transferRequest *mcmodel.TransferRequest, updateProject bool) (*mcmodel.File, *mcmodel.TransferRequestFile, error) {
	var (
		err                     error
		transferFileRequestUUID string
		trf                     *mcmodel.TransferRequestFile
	)

	if file.UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, nil, err
	}

	if transferFileRequestUUID, err = uuid.GenerateUUID(); err != nil {
		return nil, nil, err
	}

	// Wrap creation in a transaction so that both the file and the TransferRequestFile are either
	// both created, or neither is created.
	err = WithTxRetry(s.db, func(tx *gorm.DB) error {
		if result := tx.Create(file); result.Error != nil {
			return result.Error
		}

		// Create a new transfer request file entry to account for the new file
		transferRequestFile := mcmodel.TransferRequestFile{
			ProjectID:         transferRequest.ProjectID,
			OwnerID:           file.OwnerID,
			TransferRequestID: transferRequest.ID,
			Name:              file.Name,
			DirectoryID:       dirID,
			FileID:            file.ID,
			State:             "open",
			UUID:              transferFileRequestUUID,
		}

		if err := tx.Create(&transferRequestFile).Error; err != nil {
			return err
		}

		trf = &transferRequestFile

		if updateProject {
			return incrementProjectFileTypeCountAndFilesCount(tx, transferRequest.ProjectID, mime.Mime2Description(file.MimeType))
		}

		return nil
	})

	return file, trf, err
}

func (s *GormTransferRequestStor) ListDirectory(dir *mcmodel.File, transferRequest *mcmodel.TransferRequest) ([]mcmodel.File, error) {
	var files []mcmodel.File

	err := s.db.Where("directory_id = ?", dir.ID).
		Where("project_id", transferRequest.ProjectID).
		Where("deleted_at IS NULL").
		Where("dataset_id IS NULL").
		Where("current = true").
		Find(&files).Error
	if err != nil {
		return files, err
	}

	// Get files that have been uploaded
	var uploadedFiles []mcmodel.TransferRequestFile
	results := s.db.Where("directory_id = ?", dir.ID).
		Where("transfer_request_id = ?", transferRequest.ID).
		Find(&uploadedFiles)
	uploadedFilesByName := make(map[string]mcmodel.File)
	if results.Error == nil && len(uploadedFiles) != 0 {
		// Convert the files into a hashtable by name.
		for _, requestFile := range uploadedFiles {
			uploadedFilesByName[requestFile.Name] = mcmodel.File{Name: requestFile.Name}
		}
	}

	for _, fileEntry := range files {
		// Keep only uploaded files that are new
		if _, ok := uploadedFilesByName[fileEntry.Name]; ok {
			// File with name already exists in files list so delete
			delete(uploadedFilesByName, fileEntry.Name)
		}
	}

	// Now add in all the upload files that didn't already exist
	for _, fileEntry := range uploadedFilesByName {
		files = append(files, fileEntry)
	}

	return files, nil
}

func (s *GormTransferRequestStor) GetFileByPath(path string, transferRequest *mcmodel.TransferRequest) (*mcmodel.File, error) {
	// Get directory so we can use its id for lookups
	dirPath := filepath.Dir(path)
	fileName := filepath.Base(path)
	var dir mcmodel.File
	err := s.db.Where("project_id = ?", transferRequest.ProjectID).
		Where("deleted_at IS NULL").
		Where("dataset_id IS NULL").
		Where("current = true").
		Where("path = ?", dirPath).
		First(&dir).Error
	if err != nil {
		return nil, err
	}

	// We have the directory, so first check if there is an existing
	// upload for that file
	var transferRequestFile mcmodel.TransferRequestFile
	err = s.db.Preload("File.Directory").
		Where("directory_id = ?", dir.ID).
		Where("transfer_request_id = ?", transferRequest.ID).
		Where("name = ?", fileName).
		First(&transferRequestFile).Error
	if err == nil && transferRequestFile.File != nil {
		// Found file in the transfer request file
		return transferRequestFile.File, nil
	}

	// If we are here then lookup the file in the project
	var file mcmodel.File
	err = s.db.Preload("Directory").
		Where("directory_id = ?", dir.ID).
		Where("name = ?", fileName).
		Where("deleted_at IS NULL").
		Where("dataset_id IS NULL").
		Where("current = ?", true).
		First(&file).Error

	return &file, err
}

func (s *GormTransferRequestStor) GetTransferRequestForProjectAndUser(projectID, userID int) (*mcmodel.TransferRequest, error) {
	var transferRequest mcmodel.TransferRequest
	err := s.db.Where("project_id = ?", projectID).
		Where("owner_id = ?", userID).
		First(&transferRequest).Error
	if err != nil {
		return nil, err
	}

	return &transferRequest, nil
}
func (s *GormTransferRequestStor) GetTransferRequestsForProject(projectID int) ([]mcmodel.TransferRequest, error) {
	var transferRequests []mcmodel.TransferRequest

	err := s.db.Where("project_id = ?", projectID).Find(&transferRequests).Error
	if err != nil {
		return nil, err
	}

	return transferRequests, nil
}

func incrementProjectFileTypeCountAndFilesCount(db *gorm.DB, projectID int, fileTypeDescription string) error {
	var p mcmodel.Project
	// Get latest for project
	if result := db.Find(&p, projectID); result.Error != nil {
		return result.Error
	}

	fileTypes, err := p.GetFileTypes()
	if err != nil {
		return err
	}

	count, ok := fileTypes[fileTypeDescription]
	if !ok {
		fileTypes[fileTypeDescription] = 1
	} else {
		fileTypes[fileTypeDescription] = count + 1
	}

	fileTypesAsStr, err := p.ToFileTypeAsString(fileTypes)
	if err != nil {
		return err
	}

	return db.Model(&p).Updates(&mcmodel.Project{FileTypes: fileTypesAsStr, FileCount: p.FileCount + 1}).Error
}

func (s *GormTransferRequestStor) GetTransferRequestByUUID(transferUUID string) (*mcmodel.TransferRequest, error) {
	var transferRequest mcmodel.TransferRequest
	err := s.db.Preload("GlobusTransfer").
		Where("uuid = ?", transferUUID).
		First(&transferRequest).Error
	if err != nil {
		return nil, err
	}

	return &transferRequest, nil
}

func (s *GormTransferRequestStor) CloseTransferRequestByUUID(transferUUID string) error {
	return WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Model(&mcmodel.TransferRequest{}).
			Where("uuid = ?", transferUUID).
			Update("state", "closed").Error
	})
}
