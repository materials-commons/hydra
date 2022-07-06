package store

import (
	"fmt"

	"github.com/materials-commons/gomcdb/mcmodel"
)

type FakeUserStore struct {
	users []mcmodel.User
}

func NewFakeUserStore(users []mcmodel.User) *FakeUserStore {
	return &FakeUserStore{users: users}
}

func (s *FakeUserStore) GetUsersWithGlobusAccount() (error, []mcmodel.User) {
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

func (s *FakeUserStore) GetUserBySlug(slug string) (*mcmodel.User, error) {
	for _, u := range s.users {
		if u.Slug == slug {
			return &u, nil
		}
	}
	return nil, fmt.Errorf("no such user: %s", slug)
}
