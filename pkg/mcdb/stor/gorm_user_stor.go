package stor

import (
	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormUserStor struct {
	db *gorm.DB
}

func NewGormUserStor(db *gorm.DB) *GormUserStor {
	return &GormUserStor{db: db}
}

// CreateUser creates a new user.
func (s *GormUserStor) CreateUser(user *mcmodel.User) (*mcmodel.User, error) {
	var err error

	if user.UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	err = WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Create(user).Error
	})

	if err != nil {
		return nil, err
	}

	return user, nil
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

func (s *GormUserStor) GetUserByEmail(email string) (*mcmodel.User, error) {
	var user mcmodel.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *GormUserStor) GetUserByAPIToken(apitoken string) (*mcmodel.User, error) {
	var user mcmodel.User
	if err := s.db.Where("api_token = ?", apitoken).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}
