package stor

import (
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormUserStor struct {
	db *gorm.DB
}

func NewGormUserStor(db *gorm.DB) *GormUserStor {
	return &GormUserStor{db: db}
}

func (s *GormUserStor) GetUsersWithGlobusAccount() ([]mcmodel.User, error) {
	var users []mcmodel.User
	result := s.db.Where("globus_user is not null").Find(&users)
	return users, result.Error
}

func (s *GormUserStor) GetUserBySlug(slug string) (*mcmodel.User, error) {
	var user mcmodel.User
	if err := s.db.Where("slug = ?", slug).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
