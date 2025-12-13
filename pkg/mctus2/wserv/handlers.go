package wserv

import (
	"time"
)

type HandlerFunc func(msg Message, c *ClientConnection) error

var Handlers = map[string]HandlerFunc{
	"list-projects": handleListProjects,
	"heartbeat":     handleHeartbeat,

	// Setup messages for file transfer
	"transfer-init":            defaultHandler,
	"transfer-accept":          defaultHandler,
	"transfer-reject":          defaultHandler,
	"chunk-ack":                defaultHandler,
	"transfer-complete":        defaultHandler,
	"transfer-finalize":        defaultHandler,
	"transfer-resume":          defaultHandler,
	"transfer-resume-response": defaultHandler,
	"transfer-cancel":          defaultHandler,
}

func defaultHandler(msg Message, c *ClientConnection) error {
	return nil
}

type ProjectItem struct {
	Directory string `json:"directory"`
	ProjectID int    `json:"project_id"`
}

func handleListProjects(msg Message, c *ClientConnection) error {
	//fmt.Printf("handleListProjects: %+v\n", msg.Payload)
	projectsList := msg.Payload.([]interface{})
	for _, projectItem := range projectsList {
		projectItem := toProjectItem(projectItem.(map[string]interface{}))
		_ = projectItem
		//fmt.Printf("projectItem: %+v\n", projectItem)
	}

	return nil
}

func toProjectItem(project map[string]interface{}) ProjectItem {
	return ProjectItem{
		Directory: project["directory"].(string),
		ProjectID: int(project["project_id"].(float64)),
	}
}

func handleHeartbeat(msg Message, c *ClientConnection) error {
	response := Message{
		Command:   "HEARTBEAT_ACK",
		ID:        msg.ID,
		Timestamp: time.Now(),
		ClientID:  msg.ClientID,
	}
	c.Send <- response

	return nil
}
