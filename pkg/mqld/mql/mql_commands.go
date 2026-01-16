package mql

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
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
	w       http.ResponseWriter
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
	mql.interp.RegisterCommand("mql::samples", mql.samplesCommand)
	mql.interp.RegisterCommand("mql::samplesTable", mql.samplesTableCommand)
	mql.interp.RegisterCommand("mql::list-connected-clients", mql.listConnectedClientsCommand)
	mql.interp.RegisterCommand("mql::upload-file", mql.uploadFileCommand)
	mql.interp.RegisterCommand("mql::upload-directory", mql.uploadDirectoryCommand)
	mql.interp.RegisterCommand("puts", mql.putsCommand)
}

func (mql *MQLCommands) Run(query string, w http.ResponseWriter) string {
	mql.w = w
	result, err := mql.interp.Eval(query)
	if err != nil {
		return err.Error()
	}

	//fmt.Println("mql.interp.Eval = ", result)
	return result.String()
}

func (mql *MQLCommands) samplesCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
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
		items = append(items, fmt.Sprintf("name: %q id: %d owner_id: %d project_id: %d category: %s description: %q summary: %q created_at: %q",
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
		connectedClients = append(connectedClients, fmt.Sprintf("host: %s type: %s client_id: %s project_ids: %s", client.Hostname, client.Type, client.ClientID, sb.String()))
	}
	return feather.OK(connectedClients)
}

/*
"directory_path": "/local/path/to/data",
        "project_id": 1047,
        "directory_id": 100,
        "recursive": true,
        "chunk_size": 1048576 // optional
*/

func (mql *MQLCommands) uploadDirectoryCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 3 {
		return feather.Error(fmt.Errorf("upload-directory client_id directory_path recursive"))
	}
	clientID := args[0].String()
	directoryPath := args[1].String()
	recursive := parseBool(args[2].String())

	msg := wserv.Message{
		Command:   "UPLOAD_DIRECTORY",
		ID:        "mql",
		Timestamp: time.Now(),
		ClientID:  clientID,
		Payload: map[string]any{
			"directory_path": directoryPath,
			"project_id":     mql.Project.ID,
			"recursive":      recursive,
		},
	}

	mql.hub.WSManager.Broadcast() <- msg // TODO: Broadcast should take the message, rather than return the channel
	return feather.OK("submitted")
}

func (mql *MQLCommands) uploadFileCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 3 {
		return feather.Error(fmt.Errorf("upload-file client_id host_path project_path"))
	}

	clientID := args[0].String()
	hostPath := args[1].String()
	projectPath := args[2].String()

	msg := wserv.Message{
		Command:   "UPLOAD_FILE",
		ID:        "mql", // Should this be the ID of the initiating client (Web UI)?
		Timestamp: time.Now(),
		ClientID:  clientID,
		Payload: map[string]any{
			"file_path":    hostPath,
			"project_path": projectPath,
			"project_id":   mql.Project.ID,
		},
	}

	mql.hub.WSManager.Broadcast() <- msg

	return feather.OK("submitted")
}

func (mql *MQLCommands) putsCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	fmt.Fprintln(mql.w, args[0].String())
	return feather.OK("")
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

func parseBool(s string) bool {
	s = strings.TrimSpace(s)
	b, err := strconv.ParseBool(s)
	if err != nil {
		if strings.Compare(strings.ToLower(s), "true") != 0 {
			return true
		}
		return false
	}
	return b
}
