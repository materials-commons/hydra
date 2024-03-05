package stor

import (
	"errors"
	"fmt"

	"github.com/gosimple/slug"
	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type GormProjectStor struct {
	db *gorm.DB
}

func NewGormProjectStor(db *gorm.DB) *GormProjectStor {
	return &GormProjectStor{db: db}
}

func (s *GormProjectStor) CreateProject(project *mcmodel.Project) (*mcmodel.Project, error) {
	var (
		err      error
		rootUUID string
		teamUUID string
	)

	if project.FileTypes == "" {
		project.FileTypes = "{}"
	}

	if project.UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	if rootUUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	if teamUUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	slugOfName := slug.Make(project.Name)
	project.Slug = slugOfName
	slugNext := 1

	err = WithTxRetry(s.db, func(tx *gorm.DB) error {
		team := &mcmodel.Team{
			Name:    fmt.Sprintf("Team for %s", project.Name),
			OwnerID: project.OwnerID,
			UUID:    teamUUID,
		}

		// Create a team for the project and add the owner of the
		// project as the admin for the team.
		admin := mcmodel.User{ID: project.OwnerID}
		team.Members = append(team.Members, admin)
		if err = tx.Create(team).Error; err != nil {
			return err
		}

		project.TeamID = team.ID

	CreateLoop:
		for {
			err = tx.Create(project).Error
			switch {
			case err == nil:
				break CreateLoop
			case errors.Is(err, gorm.ErrForeignKeyViolated):
				// If there is a foreign key violation we assume there was a collision on the slug.
				// Add an incrementing integer to the slug name and try again.
				project.Slug = fmt.Sprintf("%s-%d", slugOfName, slugNext)
				slugNext = slugNext + 1
			default:
				return err
			}
		}

		// Create the root directory for the project.
		rootDir := &mcmodel.File{
			UUID:                 rootUUID,
			ProjectID:            project.ID,
			Name:                 "/",
			Path:                 "/",
			MimeType:             "directory",
			MediaTypeDescription: "directory",
			Current:              true,
			OwnerID:              project.OwnerID,
		}

		return tx.Create(rootDir).Error
	})

	if err != nil {
		return nil, err
	}

	return project, nil
}

func (s *GormProjectStor) AddMemberToProject(project *mcmodel.Project, user *mcmodel.User) error {
	var team mcmodel.Team

	if err := s.db.Where("id = ?", project.TeamID).Find(&team).Error; err != nil {
		return err
	}

	err := WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Model(&team).Association("Members").Append(user)
	})

	return err
}

func (s *GormProjectStor) AddAdminToProject(project *mcmodel.Project, user *mcmodel.User) error {
	var team mcmodel.Team

	if err := s.db.Where("id = ?", project.TeamID).Find(&team).Error; err != nil {
		return err
	}

	err := WithTxRetry(s.db, func(tx *gorm.DB) error {
		return tx.Model(&team).Association("Admins").Append(user)
	})

	return err
}

func (s *GormProjectStor) GetProjectByID(projectID int) (*mcmodel.Project, error) {
	var project mcmodel.Project
	err := s.db.Find(&project, projectID).Error
	if err != nil {
		return nil, err
	}

	return &project, nil
}

func (s *GormProjectStor) GetProjectBySlug(slug string) (*mcmodel.Project, error) {
	var project mcmodel.Project
	if err := s.db.Where("slug = ?", slug).First(&project).Error; err != nil {
		return nil, err
	}

	return &project, nil
}

func (s *GormProjectStor) GetProjectsForUser(userID int) ([]mcmodel.Project, error) {
	var projects []mcmodel.Project

	err := s.db.Where("team_id in (select team_id from team2admin where user_id = ?)", userID).
		Or("team_id in (select team_id from team2member where user_id = ?)", userID).
		Find(&projects).Error
	return projects, err
}

func (s *GormProjectStor) UpdateProjectSizeAndFileCount(projectID int, size int64, fileCount int) error {
	return WithTxRetry(s.db, func(tx *gorm.DB) error {
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

func (s *GormProjectStor) UpdateProjectDirectoryCount(projectID int, directoryCount int) error {
	return WithTxRetry(s.db, func(tx *gorm.DB) error {
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

func (s *GormProjectStor) UserCanAccessProject(userID, projectID int) bool {
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
