package store

import (
	"github.com/materials-commons/gomcdb/mcmodel"
	"gorm.io/gorm"
)

type GormProjectStore struct {
	db *gorm.DB
}

func NewGormProjectStore(db *gorm.DB) *GormProjectStore {
	return &GormProjectStore{db: db}
}

func (s *GormProjectStore) GetProjectByID(projectID int) (*mcmodel.Project, error) {
	var project mcmodel.Project
	err := s.db.Find(&project, projectID).Error
	if err != nil {
		return nil, err
	}

	return &project, nil
}

func (s *GormProjectStore) GetProjectBySlug(slug string) (*mcmodel.Project, error) {
	var project mcmodel.Project
	if err := s.db.Where("slug = ?", slug).First(&project).Error; err != nil {
		return nil, err
	}

	return &project, nil
}

func (s *GormProjectStore) GetProjectsForUser(userID int) ([]mcmodel.Project, error) {
	var projects []mcmodel.Project

	err := s.db.Where("team_id in (select team_id from team2admin where user_id = ?)", userID).
		Or("team_id in (select team_id from team2member where user_id = ?)", userID).
		Find(&projects).Error
	return projects, err
}

func (s *GormProjectStore) UpdateProjectSizeAndFileCount(projectID int, size int64, fileCount int) error {
	return s.withTxRetry(func(tx *gorm.DB) error {
		var p mcmodel.Project
		// Get latest project
		if result := tx.Find(&p, projectID); result.Error != nil {
			return result.Error
		}

		return tx.Model(&p).Updates(mcmodel.Project{
			FileCount: p.FileCount + fileCount,
			Size:      p.Size + size,
		}).Error
	})
}

func (s *GormProjectStore) UpdateProjectDirectoryCount(projectID int, directoryCount int) error {
	return s.withTxRetry(func(tx *gorm.DB) error {
		var p mcmodel.Project
		// Get latest project
		if result := tx.Find(&p, projectID); result.Error != nil {
			return result.Error
		}

		return tx.Model(&p).Updates(mcmodel.Project{
			DirectoryCount: p.DirectoryCount + directoryCount,
		}).Error
	})
}

func (s *GormProjectStore) UserCanAccessProject(userID, projectID int) bool {
	var project mcmodel.Project

	if err := s.db.Find(&project, projectID).Error; err != nil {
		return false
	}

	if project.OwnerID == userID {
		return true
	}

	var userCount int64
	s.db.Table("team2member").
		Where("user_id = ?", userID).
		Where("team_id = ?", project.TeamID).
		Count(&userCount)

	if userCount != 0 {
		return true
	}

	s.db.Table("team2admin").
		Where("user_id = ?", userID).
		Where("team_id = ?", project.TeamID).
		Count(&userCount)

	if userCount != 0 {
		return true
	}

	return false
}

func (s *GormProjectStore) withTxRetry(fn func(tx *gorm.DB) error) error {
	return WithTxRetryDefault(fn, s.db)
}
