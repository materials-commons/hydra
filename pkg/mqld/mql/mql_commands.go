package mql

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/feather-lang/feather"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mctus2/wserv"
	"github.com/olekukonko/tablewriter"
	"gorm.io/gorm"
)

type MQLCommands struct {
	Project *mcmodel.Project
	User    *mcmodel.User
	db      *gorm.DB
	interp  *feather.Interp
	hub     *wserv.Hub
}

func NewMQLCommands(project *mcmodel.Project, user *mcmodel.User, db *gorm.DB, interp *feather.Interp, hub *wserv.Hub) *MQLCommands {
	mql := &MQLCommands{
		Project: project,
		User:    user,
		db:      db,
		interp:  interp,
		hub:     hub,
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
	mql.interp.RegisterCommand("list-connected-clients", mql.listConnectedClientsCommand)
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

func (mql *MQLCommands) listConnectedClientsCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	//clients := mql.hub.Clients()
	//result := fmt.Sprintf("Connected clients: %d", len(clients))
	//return feather.OK(result)
	clients := mql.hub.GetConnectedClientsForUser(mql.User.ID)

	connectedClients := make([]string, 0, len(clients))

	for _, client := range clients {
		var sb strings.Builder
		sb.WriteString("{ ")
		for i, projectID := range client.Projects {
			if i > 0 {
				sb.WriteString(fmt.Sprintf(" %d", projectID))
			} else {
				sb.WriteString(fmt.Sprintf("%d", projectID))
			}
		}
		sb.WriteString(" }")
		connectedClients = append(connectedClients, fmt.Sprintf("host %s type %s client_id %s project_ids %s", client.Hostname, client.Type, client.ClientID, sb.String()))
	}
	return feather.OK(connectedClients)
}

func (mql *MQLCommands) uploadFileCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 4 {
		return feather.Error(fmt.Errorf("upload-file project_id host host_path project_path"))
	}
	return feather.OK("uploadFileCommand")
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
