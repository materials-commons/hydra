package store

import (
	"github.com/materials-commons/gomcdb/mcmodel"
	"gorm.io/gorm"
)

type GormUserStore struct {
	db *gorm.DB
}

func NewGormUserStore(db *gorm.DB) *GormUserStore {
	return &GormUserStore{db: db}
}

func (s *GormUserStore) GetUsersWithGlobusAccount() ([]mcmodel.User, error) {
	var users []mcmodel.User
	result := s.db.Where("globus_user is not null").Find(&users)
	return users, result.Error
}

func (s *GormUserStore) GetUserBySlug(slug string) (*mcmodel.User, error) {
	var user mcmodel.User
	if err := s.db.Where("slug = ?", slug).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
