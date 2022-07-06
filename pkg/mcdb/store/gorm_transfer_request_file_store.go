package store

import (
	"path/filepath"

	"github.com/apex/log"
	"github.com/materials-commons/gomcdb/mcmodel"
	"gorm.io/gorm"
)

type GormTransferRequestFileStore struct {
	db        *gorm.DB
	fileStore *GormFileStore
}

func NewGormTransferRequestFileStore(db *gorm.DB) *GormTransferRequestFileStore {
	return &GormTransferRequestFileStore{db: db, fileStore: NewGormFileStore(db, "")}
}

func (s *GormTransferRequestFileStore) DeleteTransferFileRequestByPath(ownerID, projectID int, path string) error {
	dirPath := filepath.Dir(path)
	fileName := filepath.Base(path)

	dir, err := s.fileStore.GetDirByPath(projectID, dirPath)
	if err != nil {
		log.Errorf("Unable to find directory for path %s in project %d: %s", dirPath, projectID, err)
		return err
	}

	return s.withTxRetry(func(tx *gorm.DB) error {
		return tx.
			Where("project_id = ?", projectID).
			Where("owner_id = ?", ownerID).
			Where("directory_id = ?", dir.ID).
			Where("name = ?", fileName).
			Delete(mcmodel.TransferRequestFile{}).Error
	})
}

func (s *GormTransferRequestFileStore) GetTransferFileRequestByPath(ownerID, projectID int, path string) (*mcmodel.TransferRequestFile, error) {
	dirPath := filepath.Dir(path)
	fileName := filepath.Base(path)

	dir, err := s.fileStore.GetDirByPath(projectID, dirPath)
	if err != nil {
		log.Errorf("Unable to find directory for path %s in project %d: %s", dirPath, projectID, err)
		return nil, err
	}

	var transferRequestFile mcmodel.TransferRequestFile
	err = s.db.
		Where("project_id = ?", projectID).
		Where("owner_id = ?", ownerID).
		Where("directory_id = ?", dir.ID).
		Where("name = ?", fileName).
		First(&transferRequestFile).Error
	return &transferRequestFile, err
}

func (s *GormTransferRequestFileStore) DeleteTransferRequestFile(transferRequestFile *mcmodel.TransferRequestFile) error {
	return s.withTxRetry(func(tx *gorm.DB) error {
		return tx.Delete(transferRequestFile).Error
	})
}

func (s *GormTransferRequestFileStore) withTxRetry(fn func(tx *gorm.DB) error) error {
	return WithTxRetryDefault(fn, s.db)
}
