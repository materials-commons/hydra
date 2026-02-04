package mql

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/feather-lang/feather"
	"github.com/materials-commons/hydra/pkg/decoder"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
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
	mql.interp.RegisterCommand("samples", mql.samplesCommand)
	mql.interp.RegisterCommand("computations", mql.notImplementedYetCommand)
	mql.interp.RegisterCommand("processes", mql.notImplementedYetCommand)
	mql.interp.RegisterCommand("create-sample", mql.createSampleCommand)
	mql.interp.RegisterCommand("create-process", mql.notImplementedYetCommand)
	mql.interp.RegisterCommand("create-computation", mql.notImplementedYetCommand)
	mql.interp.RegisterCommand("add-process-step", mql.notImplementedYetCommand)
	mql.interp.RegisterCommand("samplesTable", mql.samplesTableCommand)
	mql.interp.RegisterCommand("list-connected-clients", mql.listConnectedClientsCommand)
	mql.interp.RegisterCommand("upload-file", mql.uploadFileCommand)
	mql.interp.RegisterCommand("upload-directory", mql.uploadDirectoryCommand)
	mql.interp.RegisterCommand("ls", mql.lsCommand)
	mql.interp.RegisterCommand("ls-projects", mql.lsProjectsCommand)
	mql.interp.RegisterCommand("download-file", mql.downloadFileCommand)
	mql.interp.RegisterCommand("download-directory", mql.downloadDirectoryCommand)
	mql.interp.RegisterCommand("puts", mql.putsCommand)
}

func (mql *MQLCommands) Run(query string, w http.ResponseWriter) string {
	mql.w = w
	result, err := mql.interp.Eval(query)
	if err != nil {
		return err.Error()
	}

	return result.String()
}

func (mql *MQLCommands) loadPrelude() {
	mql.interp.Eval("")
}

func (mql *MQLCommands) notImplementedYetCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	return feather.Error("not implemented yet")
}

func (mql *MQLCommands) createSampleCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 1 {
		return feather.Error(fmt.Errorf("create-sample dict"))
	}

	dict, err := mql.toDict(args[0])
	if err != nil {
		return feather.Error(err)
	}

	m := dict.Items

	name, ok := m["name:"]
	if !ok {
		return feather.Error(fmt.Errorf("create-sample dict must contain 'name' string"))
	}

	if name.String() == "" {
		return feather.Error(fmt.Errorf("create-sample dict 'name' cannot be empty"))
	}

	desc := ""
	if d, ok := m["description:"]; ok {
		desc = d.String()
	}

	summary := ""
	if s, ok := m["summary:"]; ok {
		summary = s.String()
	}

	entityStor := stor.NewGormEntityStor(mql.db) // TODO: Allocate this into MQLCommands

	entity := &mcmodel.Entity{
		Name:        name.String(),
		Description: desc,
		Summary:     summary,
		Category:    "experimental",
		ProjectID:   mql.Project.ID,
		OwnerID:     mql.User.ID,
	}

	entity, err = entityStor.CreateEntity(entity)
	if err != nil {
		return feather.Error(err)
	}

	e := fmt.Sprintf("name: %q id: %d owner_id: %d project_id: %d category: %s description: %q summary: %q created_at: %q",
		entity.Name, entity.ID, entity.OwnerID, entity.ProjectID, entity.Category, entity.Description, entity.Summary, entity.CreatedAt.Format(time.DateOnly))

	return feather.OK(e)
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

	return feather.OK(ToTclString(items))
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
	return feather.OK(ToTclString(connectedClients))
}

func (mql *MQLCommands) uploadDirectoryCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) > 4 || len(args) < 3 {
		return feather.Error(fmt.Errorf("upload-directory client_id project_path recursive [directory_path]"))
	}

	clientID := args[0].String()
	payload := make(map[string]any)
	payload["project_id"] = mql.Project.ID
	payload["mc_project_path"] = args[1].String()
	payload["recursive"] = parseBool(args[2].String())
	if len(args) == 4 {
		payload["local_directory_path"] = args[3].String()
	}

	msg := wserv.Message{
		Command:   "UPLOAD_DIRECTORY",
		ID:        "mql",
		Timestamp: time.Now(),
		ClientID:  clientID,
		Payload:   payload,
	}

	mql.hub.WSManager.Broadcast(msg)
	return feather.OK("submitted")
}

func (mql *MQLCommands) uploadFileCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) > 3 || len(args) < 2 {
		return feather.Error(fmt.Errorf("upload-file client_id project_path [host_path]"))
	}

	clientID := args[0].String()
	payload := make(map[string]any)
	payload["project_id"] = mql.Project.ID
	payload["project_path"] = args[1].String()
	if len(args) == 3 {
		payload["file_path"] = args[2].String()
	}

	msg := wserv.Message{
		Command:   "UPLOAD_FILE",
		ID:        "mql", // Should this be the ID of the initiating client (Web UI)?
		Timestamp: time.Now(),
		ClientID:  clientID,
		Payload:   payload,
	}

	mql.hub.WSManager.Broadcast(msg)

	return feather.OK("submitted")
}

func (mql *MQLCommands) downloadFileCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) > 3 || len(args) < 2 {
		return feather.Error(fmt.Errorf("download-file client_id project_path [host_path]"))
	}

	clientID := args[0].String()
	projectPath := args[1].String()
	hostPath := ""
	if len(args) == 3 {
		hostPath = args[2].String()
	}

	f, err := mql.hub.FileStor.GetFileByPath(mql.Project.ID, projectPath)
	if err != nil {
		return feather.Error(err)
	}

	if err := mql.sendDownloadFile(f, clientID, projectPath, hostPath); err != nil {
		log.Errorf("Error sending upload file: %v", err)
		return feather.Error(err)
	}

	return feather.OK("submitted")
}

func (mql *MQLCommands) downloadDirectoryCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) > 3 || len(args) < 2 {
		return feather.Error(fmt.Errorf("download-file client_id project_path [host_path]"))
	}

	clientID := args[0].String()
	projectID := mql.Project.ID
	projectPath := args[1].String()

	hostPath := ""
	if len(args) == 3 {
		hostPath = args[2].String()
	}

	files, err := mql.hub.FileStor.ListDirectoryByPath(projectID, projectPath)
	if err != nil {
		return feather.Error(err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if err := mql.sendDownloadFile(&file, clientID, projectPath, hostPath); err != nil {
			log.Errorf("Error sending download file: %v", err)
		}
	}

	return feather.OK("submitted")
}

func (mql *MQLCommands) sendDownloadFile(f *mcmodel.File, clientID, projectPath, hostPath string) error {
	//fmt.Println("sendDownloadFile: ", f.Name, " f.ID =", f.ID)
	payload := make(map[string]any)
	payload["project_id"] = mql.Project.ID
	if hostPath != "" {
		payload["file_path"] = hostPath
	} else {
		payload["project_path"] = projectPath
	}

	payload["file_id"] = f.ID
	payload["size"] = f.Size
	payload["checksum"] = f.Checksum

	msg := wserv.Message{
		Command:   "DOWNLOAD_FILE",
		ID:        "mql", // Should this be the ID of the initiating client (Web UI)?
		Timestamp: time.Now(),
		ClientID:  clientID,
		Payload:   payload,
	}

	mql.hub.WSManager.Broadcast(msg)
	return nil
}

/*
"name": entry.name,

	"path": entry.as_posix(),
	"type": "directory" if entry.is_dir() else "file",
	"size": stat.st_size,
	"mtime": stat.st_mtime,
	"ctime": stat.st_ctime
*/
type lsResponse struct {
	Name  string    `json:"name"`
	Path  string    `json:"path"`
	Type  string    `json:"type"`
	Size  int64     `json:"size"`
	Mtime time.Time `json:"mtime"`
	Ctime time.Time `json:"ctime"`
}

func (mql *MQLCommands) lsCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 2 {
		return feather.Error(fmt.Errorf("ls client_id directory_path"))
	}
	clientID := args[0].String()

	req, err := mql.hub.RequestResponse().CreateRequest(clientID, mql.User.ID, "LIST_FILES", 20*time.Second)
	if err != nil {
		return feather.Error(fmt.Errorf("failed to create request: %v", err))
	}

	msg := wserv.Message{
		Command:   "LIST_DIRECTORY",
		ID:        "mql",
		Timestamp: time.Now(),
		ClientID:  clientID,
		Payload:   map[string]any{"request_id": req.RequestID, "directory_path": args[1].String()},
	}
	mql.hub.WSManager.Broadcast(msg)

	resp, err := mql.hub.RequestResponse().WaitForResponse(req)
	if err != nil {
		return feather.Error(fmt.Errorf("failed to wait for response: %v", err))
	}

	payload, ok := resp.Payload.(map[string]any)
	if !ok {
		return feather.Error(fmt.Errorf("failed to cast payload to map[string]any: %v", err))
	}

	files, ok := payload["files"].([]any)
	if !ok {
		return feather.Error(fmt.Errorf("failed to cast payload to []any: %v", err))
	}

	var items []string
	for _, item := range files {
		i := item.(map[string]any)
		lsItem, err := decoder.DecodeMapStrict[lsResponse](i)
		if err != nil {
			return feather.Error(fmt.Errorf("failed to decode lsResponse: %v", err))
		}
		items = append(items, fmt.Sprintf("name: %q path: %q type: %q size: %d mtime: %s ctime: %s",
			lsItem.Name, lsItem.Path, lsItem.Type, lsItem.Size,
			lsItem.Mtime.Format(time.RFC3339), lsItem.Ctime.Format(time.RFC3339)))
	}
	return feather.OK(ToTclString(items))
}

type lsProjectItem struct {
	ID            int    `json:"project_id"`
	Name          string `json:"directory"`
	DirectoryPath string `json:"project_dir_path"`
}

func (mql *MQLCommands) lsProjectsCommand(i *feather.Interp, cmd *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 1 {
		return feather.Error(fmt.Errorf("ls-projects client_id"))
	}

	clientID := args[0].String()

	req, err := mql.hub.RequestResponse().CreateRequest(clientID, mql.User.ID, "LIST_PROJECTS", 20*time.Second)
	if err != nil {
		return feather.Error(fmt.Errorf("failed to create request: %v", err))
	}

	msg := wserv.Message{
		Command:   "LIST_PROJECTS",
		ID:        "mql",
		Timestamp: time.Now(),
		ClientID:  clientID,
		Payload:   map[string]any{"request_id": req.RequestID},
	}

	mql.hub.WSManager.Broadcast(msg)

	resp, err := mql.hub.RequestResponse().WaitForResponse(req)
	if err != nil {
		return feather.Error(fmt.Errorf("failed to wait for response: %v", err))
	}

	payload, ok := resp.Payload.(map[string]any)
	if !ok {
		return feather.Error(fmt.Errorf("failed to cast payload to map[string]any: %v", err))
	}

	projects, ok := payload["projects"].([]any)
	if !ok {
		return feather.Error(fmt.Errorf("failed to cast payload['projects'] to []map[string]any: %v", err))
	}

	var items []string
	for _, item := range projects {
		p := item.(map[string]any)
		pItem, err := decoder.DecodeMapStrict[lsProjectItem](p)
		if err != nil {
			return feather.Error(fmt.Errorf("failed to decode lsProjectItem: %v", err))
		}
		items = append(items, fmt.Sprintf("id: %d name: %q host_path: %q", pItem.ID, pItem.Name, pItem.DirectoryPath))
	}
	return feather.OK(ToTclString(items))
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
	cmd := fmt.Sprintf("dict create %s", item)
	r, err := mql.interp.Eval(cmd)
	if err != nil {
		return nil, err
	}
	return r.Dict()
}

func parseBool(s string) bool {
	s = strings.TrimSpace(s)
	b, err := strconv.ParseBool(s)
	if err != nil {
		if strings.Compare(strings.ToLower(s), "yes") == 0 {
			return true
		}
		return false
	}
	return b
}
