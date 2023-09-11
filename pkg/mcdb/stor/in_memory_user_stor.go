package stor

import (
	"fmt"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type InMemoryUserStor struct {
	users []mcmodel.User
}

func NewInMemoryUserStor(users []mcmodel.User) *InMemoryUserStor {
	return &InMemoryUserStor{users: users}
}

func (s *InMemoryUserStor) GetUsersWithGlobusAccount() (error, []mcmodel.User) {
	var users []mcmodel.User

	for _, u := range s.users {
		if u.GlobusUser != "" {
			users = append(users, u)
		}
	}

	if len(users) == 0 {
		return fmt.Errorf("no globus users"), users
	}
	return nil, users
}

func (s *InMemoryUserStor) GetUserBySlug(slug string) (*mcmodel.User, error) {
	for _, u := range s.users {
		if u.Slug == slug {
			return &u, nil
		}
	}
	return nil, fmt.Errorf("no such user: %s", slug)
}
