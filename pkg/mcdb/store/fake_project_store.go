package store

import (
	"fmt"

	"github.com/materials-commons/gomcdb/mcmodel"
)

type FakeProjectStore struct {
	projects []mcmodel.Project

	// Allow user to set this to determine if the call to UserCanAccessProject should return true or false.
	// It defaults to true (user can access) (See NewFakeProjectStore constructor)
	UserCanAccess bool
}

func NewFakeProjectStore(projects []mcmodel.Project) *FakeProjectStore {
	return &FakeProjectStore{projects: projects, UserCanAccess: true}
}

func (s *FakeProjectStore) GetProjectByID(projectID int) (*mcmodel.Project, error) {
	for _, p := range s.projects {
		if p.ID == projectID {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("no such project: %d", projectID)
}

func (s *FakeProjectStore) GetProjectBySlug(slug string) (*mcmodel.Project, error) {
	for _, p := range s.projects {
		if p.Slug == slug {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("no such project: %s", slug)
}

func (s *FakeProjectStore) GetProjectsForUser(userID int) ([]mcmodel.Project, error) {
	var projects []mcmodel.Project
	for _, p := range s.projects {
		if p.OwnerID == userID {
			projects = append(projects, p)
		}
	}

	if len(projects) == 0 {
		return projects, fmt.Errorf("user has no projects")
	}
	return projects, nil
}

func (s *FakeProjectStore) UpdateProjectSizeAndFileCount(projectID int, size int64, fileCount int) error {
	for i, _ := range s.projects {
		if s.projects[i].ID == projectID {
			s.projects[i].Size = s.projects[i].Size + size
			s.projects[i].FileCount = s.projects[i].FileCount + fileCount
			return nil
		}
	}
	return fmt.Errorf("no such project: %d", projectID)
}

func (s *FakeProjectStore) UpdateProjectDirectoryCount(projectID int, directoryCount int) error {
	for i, _ := range s.projects {
		if s.projects[i].ID == projectID {
			s.projects[i].DirectoryCount = s.projects[i].DirectoryCount + directoryCount
			return nil
		}
	}
	return fmt.Errorf("no such project: %d", projectID)
}

func (s *FakeProjectStore) UserCanAccessProject(userID, projectID int) bool {
	return s.UserCanAccess
}
