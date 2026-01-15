package mql

import (
	"bytes"
	"fmt"
	"time"

	"github.com/feather-lang/feather"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/olekukonko/tablewriter"
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
	mql.interp.RegisterCommand("samplesTable", mql.samplesTableCommand)
}

func (mql *MQLCommands) Run(query string) string {
	result, err := mql.interp.Eval(query)
	if err != nil {
		return err.Error()
	}

	//fmt.Println("mql.interp.Eval = ", result)
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

	var items []string
	for _, sample := range samples {
		items = append(items, fmt.Sprintf("name %q id %d owner_id %d project_id %d category %s description %q summary %q created_at %q",
			sample.Name, sample.ID, sample.OwnerID, sample.ProjectID, sample.Category, sample.Description, sample.Summary, sample.CreatedAt.Format(time.DateOnly)))
	}

	return feather.OK(items)
}

func (mql *MQLCommands) samplesTableCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {

	//fmt.Println("type of =", args[0].Type())
	//
	//samples, err := mql.toList(args[0])
	//if err != nil {
	//	return feather.Error(err)
	//}
	//
	//for _, sampleObj := range samples {
	//	sample, err := mql.toDict(sampleObj)
	//	if err != nil {
	//		return feather.Error(err)
	//	}
	//	fmt.Println(sample)
	//}

	data := [][]string{
		{"Package", "Version", "Status"},
		{"tablewriter", "v0.0.5", "legacy"},
		{"tablewriter", "v1.1.2", "latest"},
	}

	buf := new(bytes.Buffer)
	table := tablewriter.NewWriter(buf)
	buf.WriteString("\n")
	defer table.Close()
	table.Header(data[0])
	table.Bulk(data[1:])
	table.Render()
	result := buf.String()
	return feather.OK(result)
}

func (mql *MQLCommands) toList(what *feather.Obj) ([]*feather.Obj, error) {
	r, err := mql.interp.Eval(fmt.Sprintf("list %s", what))
	if err != nil {
		return nil, err
	}
	return r.List()
}

func (mql *MQLCommands) toDict(item *feather.Obj) (*feather.DictType, error) {
	r, err := mql.interp.Eval(fmt.Sprintf("dict create %s", item))
	if err != nil {
		return nil, err
	}
	return r.Dict()
}
