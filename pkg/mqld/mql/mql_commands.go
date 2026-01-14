package mql

import (
	"encoding/json"
	"fmt"

	"github.com/feather-lang/feather"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type MQLCommands struct {
	Project *mcmodel.Project
	User    *mcmodel.User
	db      *gorm.DB
	interp  *feather.Interp
}

func NewMQLCommands(project *mcmodel.Project, user *mcmodel.User, db *gorm.DB, interp *feather.Interp) *MQLCommands {
	mql := &MQLCommands{
		Project: project,
		User:    user,
		db:      db,
		interp:  interp,
	}

	mql.registerCommands()
	return mql
}

type MyObj struct {
	ID          int
	Name        string
	AnotherID   int
	AnotherName string
}

func (mql *MQLCommands) registerCommands() {
	mql.interp.RegisterCommand("querySamples", mql.querySamplesCommand)
	mql.interp.RegisterCommand("obj", func(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
		o := MyObj{
			Name:        "obj",
			AnotherName: "anotherObj",
			ID:          1,
			AnotherID:   2,
		}

		data, _ := json.Marshal(o)
		var res map[string]any
		_ = json.Unmarshal(data, &res)

		return feather.OK(res)
	})
}

func (mql *MQLCommands) Run(query string) string {
	result, err := mql.interp.Eval(query)
	if err != nil {
		return err.Error()
	}

	fmt.Println("mql.interp.Eval = ", result)
	return result.String()
}

func (mql *MQLCommands) querySamplesCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	var samples []mcmodel.Entity
	err := mql.db.Where("project_id = ?", mql.Project.ID).
		Where("category = ?", "experimental").
		Limit(2).
		Find(&samples).Error
	if err != nil {
		return feather.Error(err)
	}

	data, _ := json.Marshal(samples)
	var res []map[string]any
	_ = json.Unmarshal(data, &res)

	//fmt.Printf("+%v\n", res)
	return feather.OK(res)
}
