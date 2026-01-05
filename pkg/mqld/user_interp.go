package mqld

import (
	"sync"

	"github.com/apex/log"
	"github.com/feather-lang/feather"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type UserInterp struct {
	Interp    *feather.Interp
	UserID    int
	ProjectID int
}

type UserInterps struct {
	InterpsByProject sync.Map // Interpreters by ProjectID
	User             *mcmodel.User
}

func NewUserInterps(User *mcmodel.User) *UserInterps {
	return &UserInterps{
		User: User,
	}
}

func (u *UserInterps) GetInterpsForProject(projectID int) *UserInterp {
	interp, ok := u.InterpsByProject.Load(projectID)
	if !ok {
		userInterp := NewUserInterp(projectID, u.User.ID)
		interp, ok = u.InterpsByProject.LoadOrStore(projectID, userInterp)
		if !ok {
			log.Errorf("Failed to store new user interp for project (%d)", projectID)
			return nil
		}
	}

	return interp.(*UserInterp)
}

func NewUserInterp(projectID, userID int) *UserInterp {
	return &UserInterp{
		Interp:    feather.New(),
		UserID:    userID,
		ProjectID: projectID,
	}

	// TODO: Load commands and scripts that the user created
}
