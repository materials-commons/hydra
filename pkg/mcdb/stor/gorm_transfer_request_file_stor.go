package stor

import (
	"path/filepath"

	"github.com/apex/log"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormTransferRequestFileStor struct {
	db        *gorm.DB
	fileStore *GormFileStor
}

func NewGormTransferRequestFileStor(db *gorm.DB) *GormTransferRequestFileStor {
	return &GormTransferRequestFileStor{db: db, fileStore: NewGormFileStor(db, "")}
}

func (s *GormTransferRequestFileStor) DeleteTransferFileRequestByPath(ownerID, projectID int, path string) error {
	dirPath := filepath.Dir(path)
	fileName := filepath.Base(path)

	dir, err := s.fileStore.GetDirByPath(projectID, dirPath)
	if err != nil {
		log.Errorf("Unable to find directory for path %s in project %d: %s", dirPath, projectID, err)
		return err
	}

	return WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.
			Where("project_id = ?", projectID).
			Where("owner_id = ?", ownerID).
			Where("directory_id = ?", dir.ID).
			Where("name = ?", fileName).
			Delete(mcmodel.TransferRequestFile{}).Error
	})
}

func (s *GormTransferRequestFileStor) GetTransferFileRequestByPath(ownerID, projectID int, path string) (*mcmodel.TransferRequestFile, error) {
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

func (s *GormTransferRequestFileStor) DeleteTransferRequestFile(transferRequestFile *mcmodel.TransferRequestFile) error {
	return WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Delete(transferRequestFile).Error
	})
}
