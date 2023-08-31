package stor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormFileStor struct {
	db       *gorm.DB
	mcfsRoot string
}

func NewGormFileStor(db *gorm.DB, mcfsRoot string) *GormFileStor {
	return &GormFileStor{db: db, mcfsRoot: mcfsRoot}
}

func (s *GormFileStor) GetFileByID(fileID int) (*mcmodel.File, error) {
	var file mcmodel.File
	if err := s.db.Find(&file, fileID).Error; err != nil {
		return nil, err
	}

	return &file, nil
}

func (s *GormFileStor) GetFileByUUID(fileUUID string) (*mcmodel.File, error) {
	var file mcmodel.File
	if err := s.db.Where("uuid = ?", fileUUID).First(&file).Error; err != nil {
		return nil, err
	}

	return &file, nil
}

// UpdateMetadataForFileAndProject updates the metadata and project meta data for a file
func (s *GormFileStor) UpdateMetadataForFileAndProject(file *mcmodel.File, checksum string, totalBytes int64) error {
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

		// Now we can update the meta data on the current file. This includes, the size, current, and if there is
		// a new computed checksum, also update the checksum field.
		fileMetadata := mcmodel.File{
			Size:     uint64(finfo.Size()),
			Current:  true,
			Checksum: checksum,
		}

		if err := tx.Model(file).Updates(&fileMetadata).Error; err != nil {
			return err
		}

		var project mcmodel.Project

		if result := tx.Find(&project, file.ProjectID); result.Error != nil {
			return result.Error
		}

		return tx.Model(&project).Updates(&mcmodel.Project{Size: project.Size + totalBytes}).Error
	})
}

func (s *GormFileStor) CreateFile(name string, projectID, directoryID, ownerID int, mimeType string) (*mcmodel.File, error) {
	newFile := &mcmodel.File{
		ProjectID:   projectID,
		Name:        name,
		DirectoryID: directoryID,
		Size:        0,
		Checksum:    "",
		MimeType:    mimeType,
		OwnerID:     ownerID,
		Current:     false,
	}

	var err error

	if newFile.UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	err = WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Create(newFile).Error
	})

	return newFile, err
}

func (s *GormFileStor) GetDirByPath(projectID int, path string) (*mcmodel.File, error) {
	return findDirByPath(s.db, projectID, path)
}

func findDirByPath(db *gorm.DB, projectID int, path string) (*mcmodel.File, error) {
	var dir mcmodel.File
	err := db.Preload("Directory").
		Where("project_id = ?", projectID).
		Where("path = ?", path).
		Where("deleted_at IS NULL").
		Where("dataset_id IS NULL").
		First(&dir).Error
	if err != nil {
		//log.Errorf("Failed looking up directory in project %d, path %s: %s", projectID, path, err)
		return nil, err
	}

	return &dir, nil
}

func (s *GormFileStor) CreateDirectory(parentDirID, projectID, ownerID int, path, name string) (*mcmodel.File, error) {
	var dir mcmodel.File
	err := WithTxRetry(s.db, func(tx *gorm.DB) error {
		err := tx.Where("path = ", path).
			Where("deleted_at IS NULL").
			Where("dataset_id IS NULL").
			Where("project_id = ?", projectID).
			Find(&dir).Error
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			// directory already exists no need to create
			return nil
		}

		var project mcmodel.Project
		if result := tx.Find(&project, projectID); result.Error != nil {
			return result.Error
		}

		dir = mcmodel.File{
			OwnerID:              ownerID,
			MimeType:             "directory",
			MediaTypeDescription: "directory",
			DirectoryID:          parentDirID,
			Current:              true,
			Path:                 path,
			ProjectID:            projectID,
			Name:                 name,
		}

		if dir.UUID, err = uuid.GenerateUUID(); err != nil {
			return err
		}

		if err := tx.Create(&dir).Error; err != nil {
			return err
		}

		return tx.Model(&project).Updates(&mcmodel.Project{DirectoryCount: project.DirectoryCount + 1}).Error
	})

	return &dir, err
}

func (s *GormFileStor) CreateDirIfNotExists(parentDirID int, path, name string, projectID, ownerID int) (*mcmodel.File, error) {
	var (
		dir *mcmodel.File
		err error
	)

	err = WithTxRetry(s.db, func(tx *gorm.DB) error {
		dir, err = findDirByPath(tx, projectID, path)
		if err == nil {
			// dir found
			return nil
		}

		dir = &mcmodel.File{
			OwnerID:              ownerID,
			MimeType:             "directory",
			MediaTypeDescription: "directory",
			DirectoryID:          parentDirID,
			Current:              true,
			Path:                 path,
			ProjectID:            projectID,
			Name:                 name,
		}

		if dir.UUID, err = uuid.GenerateUUID(); err != nil {
			return err
		}

		if err := tx.Create(dir).Error; err != nil {
			return err
		}

		project := mcmodel.Project{ID: projectID}

		return tx.Model(&project).Updates(&mcmodel.Project{DirectoryCount: project.DirectoryCount + 1}).Error
	})

	return dir, err
}

func (s *GormFileStor) ListDirectoryByPath(projectID int, path string) ([]mcmodel.File, error) {
	dir, err := s.GetDirByPath(projectID, path)
	if err != nil {
		return nil, err
	}

	var files []mcmodel.File

	err = s.db.Where("directory_id = ?", dir.ID).
		Where("project_id", projectID).
		Where("deleted_at IS NULL").
		Where("dataset_id IS NULL").
		Where("current = true").
		Find(&files).Error

	return files, err
}

// GetOrCreateDirPath will create all entries in the directory path if the path doesn't exist
func (s *GormFileStor) GetOrCreateDirPath(projectID, ownerID int, path string) (*mcmodel.File, error) {
	dir, err := s.GetDirByPath(projectID, path)
	if err == nil {
		// If we are here then the path was found, and we have nothing left to do.
		return dir, nil
	}

	// If we are here then directory wasn't found. At this point we don't know how many levels deep we have
	// to create directories. The common case is that this directory doesn't exist, but the parent does. Let's
	// check that case since it saves us a lot of work.
	parentPath := filepath.Dir(path)
	parentDir, err := s.GetDirByPath(projectID, parentPath)
	if err == nil {
		// Ok, the parent exists, so just create the child of the parent (ie, the complete path) and return
		// the created directory.
		return s.CreateDirectory(parentDir.ID, projectID, ownerID, path, filepath.Base(path))
	}

	// If we are here then the path didn't exist and the parent didn't exist so now we are going to traverse
	// upwards constructing as we go. The way we do this is to split the path, retrieve the root, and then just
	// start appending each entry of the path on, checking if it exists and if it doesn't then create it.

	// Start with root and then go from there
	parentDir, err = s.GetDirByPath(projectID, "/")
	if err != nil {
		return nil, err
	}

	pathParts := strings.Split(path, "/")
	currentPath := "/"
	for _, pathPart := range pathParts[1:] {
		currentPath = filepath.Join(currentPath, pathPart)
		dir, err = s.CreateDirIfNotExists(parentDir.ID, currentPath, filepath.Base(currentPath), projectID, ownerID)
		if err != nil {
			return nil, err
		}
		parentDir = dir
	}

	return dir, nil
}

func (s *GormFileStor) GetFileByPath(projectID int, path string) (*mcmodel.File, error) {
	if path == "/" {
		return s.GetDirByPath(projectID, path)
	}

	dirPath := filepath.Dir(path)
	dir, err := s.GetDirByPath(projectID, dirPath)
	if err != nil {
		return nil, err
	}

	var file mcmodel.File
	err = s.db.Preload("Directory").
		Where("directory_id = ?", dir.ID).
		Where("name = ?", filepath.Base(path)).
		Where("deleted_at IS NULL").
		Where("dataset_id IS NULL").
		Where("current = ?", true).
		First(&file).Error
	if err != nil {
		return nil, err
	}

	return &file, nil
}

func (s *GormFileStor) UpdateFileUses(file *mcmodel.File, uuid string, fileID int) error {
	return WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Model(file).Updates(mcmodel.File{
			UsesUUID: uuid,
			UsesID:   fileID,
		}).Error
	})
}

func (s *GormFileStor) PointAtExistingIfExists(file *mcmodel.File) (bool, error) {
	switched := false // Set to true in withTxRetry if an existing file with same checksum is found
	err := WithTxRetry(s.db, func(tx *gorm.DB) error {
		var matched mcmodel.File
		err := tx.Where("checksum = ?", file.Checksum).
			Where("deleted_at IS NULL").
			Where("id <> ?", file.ID).
			First(&matched).Error
		if err == nil {
			// found a match
			switched = true
			usesUUID := matched.UUIDForUses()
			usesID := matched.IDForUses()
			return tx.Model(file).Updates(mcmodel.File{
				UsesUUID: usesUUID,
				UsesID:   usesID,
			}).Error
		}
		return nil
	})

	return switched, err
}

// DoneWritingToFile is called when a file has been opened for writing and the caller is finished writing to it.
// It consolidates common steps such as updating metadata, switching to point to a file that already exists with
// the same checksum, and queuing the file for conversion (if needed).
func (s *GormFileStor) DoneWritingToFile(file *mcmodel.File, checksum string, size int64, conversionStore ConversionStor) (bool, error) {
	var (
		fileSwitched = false
		err          error
	)

	if err = s.UpdateMetadataForFileAndProject(file, checksum, size); err != nil {
		log.Errorf("failure updating file (%d) and project (%d) metadata: %s", file.ID, file.ProjectID, err)
		return false, err
	}

	// Check if there is a file with matching checksum, and if so have the file point at it.
	if fileSwitched, err = s.PointAtExistingIfExists(file); err != nil {
		// Some error returned, so file wasn't switched.
		return false, err
	}

	// Check if file type is one we do a conversion on to make viewable on the web, and if it is
	// then schedule a conversion to run.
	if file.IsConvertible() {
		// Queue up a conversion job
		if _, err = conversionStore.AddFileToConvert(file); err != nil {
			log.Errorf("failed adding file %d to be converted: %s", file.ID, err)
			return fileSwitched, err
		}
	}

	return fileSwitched, nil
}
