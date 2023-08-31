package stor

import (
	"fmt"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type FakeUserStor struct {
	users []mcmodel.User
}

func NewFakeUserStor(users []mcmodel.User) *FakeUserStor {
	return &FakeUserStor{users: users}
}

func (s *FakeUserStor) GetUsersWithGlobusAccount() (error, []mcmodel.User) {
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

func (s *FakeUserStor) GetUserBySlug(slug string) (*mcmodel.User, error) {
	for _, u := range s.users {
		if u.Slug == slug {
			return &u, nil
		}
	}
	return nil, fmt.Errorf("no such user: %s", slug)
}
