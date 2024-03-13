package mcdav

import (
	"errors"
	"sync"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/webdav"
	"golang.org/x/crypto/bcrypt"
)

type UserEntry struct {
	Server   *webdav.Handler
	User     *mcmodel.User
	password string
}

type Users struct {
	userStor     stor.UserStor
	usersByEmail sync.Map
}

var ErrInvalidUserOrPassword = errors.New("invalid user or password")

func NewUsers(userStor stor.UserStor) *Users {
	return &Users{userStor: userStor}
}

func (u *Users) GetOrCreateValidatedUser(username, password string) (*UserEntry, error) {
	entry, ok := u.usersByEmail.Load(username)

	if !ok {
		// User not found. So validate the password and create a new user entry.
		user, err := u.userStor.GetUserByEmail(username)
		if err != nil {
			return nil, ErrInvalidUserOrPassword
		}

		if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
			// Passwords don't match
			return nil, ErrInvalidUserOrPassword
		}

		// Everything looks good, create the entry and store it.
		userEntry := &UserEntry{password: password, User: user}
		u.usersByEmail.Store(username, userEntry)

		return userEntry, nil
	}

	userEntry := entry.(*UserEntry)

	if userEntry.password == password {
		// This is a shortcut. We've previously found the user, so check the password passed in,
		// and if matches what was previously passed in then we can proceed.
		return userEntry, nil
	}

	// Password doesn't match, lets see if the user changed it on the server.
	user, err := u.userStor.GetUserByEmail(username)

	if err != nil {
		return nil, ErrInvalidUserOrPassword
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrInvalidUserOrPassword
	}

	// The password was changed, so save it.
	userEntry.password = password

	return userEntry, nil
}

func (u *Users) GetUserByUsername(username string) *UserEntry {
	entry, ok := u.usersByEmail.Load(username)
	if !ok {
		return nil
	}

	return entry.(*UserEntry)
}
