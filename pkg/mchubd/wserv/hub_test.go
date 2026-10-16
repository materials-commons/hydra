package wserv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

func TestHubGetAuthTokenFromBearerHeader(t *testing.T) {
	hub := newTestHubForHubTests()

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	token, err := hub.getAuthToken(req)
	if err != nil {
		t.Fatalf("getAuthToken returned error: %v", err)
	}

	if token != "test-token" {
		t.Fatalf("token = %q, want %q", token, "test-token")
	}
}

func TestHubGetAuthTokenFromQueryString(t *testing.T) {
	hub := newTestHubForHubTests()

	req := httptest.NewRequest(http.MethodGet, "/ws?api_token=query-token", nil)

	token, err := hub.getAuthToken(req)
	if err != nil {
		t.Fatalf("getAuthToken returned error: %v", err)
	}

	if token != "query-token" {
		t.Fatalf("token = %q, want %q", token, "query-token")
	}
}

func TestHubGetAuthTokenRejectsMissingToken(t *testing.T) {
	hub := newTestHubForHubTests()

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)

	_, err := hub.getAuthToken(req)
	if err == nil {
		t.Fatalf("getAuthToken returned nil error, want error")
	}
}

func TestHubExtractTokenFromAuthHeaderRejectsInvalidFormat(t *testing.T) {
	hub := newTestHubForHubTests()

	tests := []string{
		"",
		"Basic abc123",
		"Bearer",
		"Bearer ",
		"abc123",
	}

	for _, authHeader := range tests {
		t.Run(authHeader, func(t *testing.T) {
			_, err := hub.extractTokenFromAuthHeader(authHeader)
			if err == nil {
				t.Fatalf("extractTokenFromAuthHeader(%q) returned nil error, want error", authHeader)
			}
		})
	}
}

func TestGetClientConnectionAttributesPrefersHeaders(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/ws?client_id=query-client&connection_type=query-type&hostname=query-host&projects=1,2",
		nil,
	)
	req.Header.Set("MC-Client-ID", "header-client")
	req.Header.Set("MC-Connection-Type", "header-type")
	req.Header.Set("MC-Client-Hostname", "header-host")
	req.Header.Set("MC-Client-Projects", "3,4")

	attrs := getClientConnectionAttributes(req)

	if attrs.ClientID != "header-client" {
		t.Fatalf("ClientID = %q, want %q", attrs.ClientID, "header-client")
	}
	if attrs.Type != "header-type" {
		t.Fatalf("Type = %q, want %q", attrs.Type, "header-type")
	}
	if attrs.Hostname != "header-host" {
		t.Fatalf("Hostname = %q, want %q", attrs.Hostname, "header-host")
	}
	if attrs.Projects != "3,4" {
		t.Fatalf("Projects = %q, want %q", attrs.Projects, "3,4")
	}
}

func TestGetClientConnectionAttributesFallsBackToQueryParams(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/ws?client_id=query-client&connection_type=query-type&hostname=query-host&projects=1,2",
		nil,
	)

	attrs := getClientConnectionAttributes(req)

	if attrs.ClientID != "query-client" {
		t.Fatalf("ClientID = %q, want %q", attrs.ClientID, "query-client")
	}
	if attrs.Type != "query-type" {
		t.Fatalf("Type = %q, want %q", attrs.Type, "query-type")
	}
	if attrs.Hostname != "query-host" {
		t.Fatalf("Hostname = %q, want %q", attrs.Hostname, "query-host")
	}
	if attrs.Projects != "1,2" {
		t.Fatalf("Projects = %q, want %q", attrs.Projects, "1,2")
	}
}

func TestCommaSeparatedProjectIDsToProjectsSkipsInvalidAndMissingProjects(t *testing.T) {
	hub := newTestHubForHubTests()

	projectStor := hub.ProjectStor.(*hubTestProjectStor)
	projectStor.projects[1] = &mcmodel.Project{ID: 1, Name: "Project 1"}
	projectStor.projects[3] = &mcmodel.Project{ID: 3, Name: "Project 3"}

	projects := hub.commaSeparatedProjectIDsToProjects("1, bad, 2, 3")

	if len(projects) != 2 {
		t.Fatalf("len(projects) = %d, want 2", len(projects))
	}

	if projects[0].ID != 1 {
		t.Fatalf("projects[0].ID = %d, want 1", projects[0].ID)
	}

	if projects[1].ID != 3 {
		t.Fatalf("projects[1].ID = %d, want 3", projects[1].ID)
	}
}

func TestGetOrCreateRemoteClientReturnsExistingClient(t *testing.T) {
	hub := newTestHubForHubTests()

	remoteStor := hub.RemoteClientStor.(*hubTestRemoteClientStor)
	existing := &mcmodel.RemoteClient{
		ID:       10,
		ClientID: "client-1",
		Hostname: "existing-host",
	}
	remoteStor.clientsByClientID["client-1"] = existing

	got, err := hub.getOrCreateRemoteClient(&ConnectionAttributes{
		ClientID: "client-1",
		Hostname: "new-host",
		Type:     "gocli",
	}, &mcmodel.User{ID: 99})

	if err != nil {
		t.Fatalf("getOrCreateRemoteClient returned error: %v", err)
	}

	if got != existing {
		t.Fatalf("getOrCreateRemoteClient returned different client")
	}

	if remoteStor.createdCount != 0 {
		t.Fatalf("createdCount = %d, want 0", remoteStor.createdCount)
	}
}

func TestGetOrCreateRemoteClientCreatesMissingClient(t *testing.T) {
	hub := newTestHubForHubTests()

	remoteStor := hub.RemoteClientStor.(*hubTestRemoteClientStor)

	got, err := hub.getOrCreateRemoteClient(&ConnectionAttributes{
		ClientID: "client-1",
		Hostname: "host-1",
		Type:     "gocli",
	}, &mcmodel.User{ID: 99})

	if err != nil {
		t.Fatalf("getOrCreateRemoteClient returned error: %v", err)
	}

	if got.ClientID != "client-1" {
		t.Fatalf("ClientID = %q, want %q", got.ClientID, "client-1")
	}
	if got.Hostname != "host-1" {
		t.Fatalf("Hostname = %q, want %q", got.Hostname, "host-1")
	}
	if got.Name != "host-1" {
		t.Fatalf("Name = %q, want %q", got.Name, "host-1")
	}
	if got.Type != "gocli" {
		t.Fatalf("Type = %q, want %q", got.Type, "gocli")
	}
	if got.OwnerID != 99 {
		t.Fatalf("OwnerID = %d, want 99", got.OwnerID)
	}
	if got.LastSeenAt.IsZero() {
		t.Fatalf("LastSeenAt was zero")
	}

	if remoteStor.createdCount != 1 {
		t.Fatalf("createdCount = %d, want 1", remoteStor.createdCount)
	}
}

func TestHandleSendCommandRejectsNonPost(t *testing.T) {
	hub := newTestHubForHubTests()

	req := httptest.NewRequest(http.MethodGet, "/command", nil)
	rec := httptest.NewRecorder()

	hub.HandleSendCommand(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleSendCommandRejectsMalformedJSON(t *testing.T) {
	hub := newTestHubForHubTests()

	req := httptest.NewRequest(http.MethodPost, "/command", bytes.NewBufferString("{bad json"))
	rec := httptest.NewRecorder()

	hub.HandleSendCommand(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleSendCommandRejectsMissingUserID(t *testing.T) {
	hub := newTestHubForHubTests()

	body := map[string]any{
		"client_id": "client-1",
		"command":   "TEST_COMMAND",
	}
	req := newJSONRequest(t, http.MethodPost, "/command", body)
	rec := httptest.NewRecorder()

	hub.HandleSendCommand(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleSendCommandRejectsMissingClientID(t *testing.T) {
	hub := newTestHubForHubTests()

	body := map[string]any{
		"user_id": 123,
		"command": "TEST_COMMAND",
	}
	req := newJSONRequest(t, http.MethodPost, "/command", body)
	rec := httptest.NewRecorder()

	hub.HandleSendCommand(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleSendCommandRejectsUnknownClient(t *testing.T) {
	hub := newTestHubForHubTests()

	body := map[string]any{
		"user_id":   123,
		"client_id": "missing-client",
		"command":   "TEST_COMMAND",
	}
	req := newJSONRequest(t, http.MethodPost, "/command", body)
	rec := httptest.NewRecorder()

	hub.HandleSendCommand(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleSendCommandRejectsWrongUser(t *testing.T) {
	hub := newTestHubForHubTests()
	registerHubTestClient(hub, &ClientConnection{
		ID:       "client-1",
		Type:     "gocli",
		Hostname: "host-1",
		User:     &mcmodel.User{ID: 123},
		Send:     make(chan Message, 10),
	})

	body := map[string]any{
		"user_id":   999,
		"client_id": "client-1",
		"command":   "TEST_COMMAND",
	}
	req := newJSONRequest(t, http.MethodPost, "/command", body)
	rec := httptest.NewRecorder()

	hub.HandleSendCommand(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandleSendCommandBroadcastsToClient(t *testing.T) {
	hub := newTestHubForHubTests()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := runHubForTest(ctx, hub)

	client := &ClientConnection{
		ID:       "client-1",
		Type:     "gocli",
		Hostname: "host-1",
		User:     &mcmodel.User{ID: 123},
		Send:     make(chan Message, 10),
	}
	registerHubTestClient(hub, client)

	body := map[string]any{
		"user_id":   123,
		"client_id": "client-1",
		"command":   "TEST_COMMAND",
		"payload": map[string]any{
			"value": "abc",
		},
	}
	req := newJSONRequest(t, http.MethodPost, "/command", body)
	rec := httptest.NewRecorder()

	hub.HandleSendCommand(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	select {
	case msg := <-client.Send:
		if msg.Command != "TEST_COMMAND" {
			t.Fatalf("Command = %q, want %q", msg.Command, "TEST_COMMAND")
		}
		if msg.ClientID != "client-1" {
			t.Fatalf("ClientID = %q, want %q", msg.ClientID, "client-1")
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for broadcasted message")
	}

	cancel()
	waitForHubToStop(t, done)
}

func TestHandleListClients(t *testing.T) {
	hub := newTestHubForHubTests()

	registerHubTestClient(hub, &ClientConnection{
		ID:       "client-1",
		Type:     "gocli",
		Hostname: "host-1",
		User:     &mcmodel.User{ID: 123},
		Projects: []*mcmodel.Project{
			{ID: 10},
			{ID: 20},
		},
		Send: make(chan Message, 10),
	})

	req := httptest.NewRequest(http.MethodGet, "/clients", nil)
	rec := httptest.NewRecorder()

	hub.HandleListClients(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var clients []ListClientsResp
	if err := json.NewDecoder(rec.Body).Decode(&clients); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(clients) != 1 {
		t.Fatalf("len(clients) = %d, want 1", len(clients))
	}

	if clients[0].ClientID != "client-1" {
		t.Fatalf("ClientID = %q, want %q", clients[0].ClientID, "client-1")
	}
	if clients[0].UserID != 123 {
		t.Fatalf("UserID = %d, want 123", clients[0].UserID)
	}
	if len(clients[0].Projects) != 2 || clients[0].Projects[0] != 10 || clients[0].Projects[1] != 20 {
		t.Fatalf("Projects = %#v, want []int{10, 20}", clients[0].Projects)
	}
}

func TestHandleListClientsForUser(t *testing.T) {
	hub := newTestHubForHubTests()

	registerHubTestClient(hub, &ClientConnection{
		ID:       "client-1",
		Type:     "gocli",
		Hostname: "host-1",
		User:     &mcmodel.User{ID: 123},
		Send:     make(chan Message, 10),
	})
	registerHubTestClient(hub, &ClientConnection{
		ID:       "client-2",
		Type:     "gocli",
		Hostname: "host-2",
		User:     &mcmodel.User{ID: 999},
		Send:     make(chan Message, 10),
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123/clients", nil)
	req.SetPathValue("id", "123")

	rec := httptest.NewRecorder()

	hub.HandleListClientsForUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var clients []ListClientsResp
	if err := json.NewDecoder(rec.Body).Decode(&clients); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(clients) != 1 {
		t.Fatalf("len(clients) = %d, want 1", len(clients))
	}

	if clients[0].ClientID != "client-1" {
		t.Fatalf("ClientID = %q, want %q", clients[0].ClientID, "client-1")
	}
}

func TestHandleListClientsForUserRejectsInvalidUserID(t *testing.T) {
	hub := newTestHubForHubTests()

	req := httptest.NewRequest(http.MethodGet, "/users/bad/clients", nil)
	req.SetPathValue("id", "bad")

	rec := httptest.NewRecorder()

	hub.HandleListClientsForUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRunStopsWhenContextCancelled(t *testing.T) {
	hub := newTestHubForHubTests()

	ctx, cancel := context.WithCancel(context.Background())
	done := runHubForTest(ctx, hub)

	cancel()
	waitForHubToStop(t, done)
}

func newTestHubForHubTests() *Hub {
	return NewHubWithDependencies(HubDependencies{
		WSManager:        NewWebSocketManager(),
		SSEManager:       NewSSEManager(),
		RRManager:        NewRequestResponseManager(30 * time.Second),
		UserStor:         newHubTestUserStor(),
		ProjectStor:      newHubTestProjectStor(),
		RemoteClientStor: newHubTestRemoteClientStor(),
		WSUpgrader:       &hubTestWebSocketUpgrader{},
	})
}

func runHubForTest(ctx context.Context, hub *Hub) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		hub.Run(ctx)
	}()
	return done
}

func waitForHubToStop(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("hub did not stop")
	}
}

func registerHubTestClient(hub *Hub, client *ClientConnection) {
	hub.WSManager.HandleRegister(client)
}

func newJSONRequest(t *testing.T, method string, target string, body any) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatalf("encode request body: %v", err)
	}

	req := httptest.NewRequest(method, target, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}

type hubTestWebSocketUpgrader struct {
	conn Connection
	err  error
}

func (u *hubTestWebSocketUpgrader) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (Connection, error) {
	if u.err != nil {
		return nil, u.err
	}
	if u.conn != nil {
		return u.conn, nil
	}
	return &hubTestConnection{}, nil
}

type hubTestConnection struct{}

func (c *hubTestConnection) ReadMessage() (messageType int, p []byte, err error) {
	return 0, nil, errors.New("closed")
}

func (c *hubTestConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *hubTestConnection) SetPongHandler(h func(appData string) error) {}

func (c *hubTestConnection) WriteMessage(messageType int, data []byte) error {
	return nil
}

func (c *hubTestConnection) WriteJSON(v any) error {
	return nil
}

func (c *hubTestConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *hubTestConnection) Close() error {
	return nil
}

type hubTestUserStor struct {
	usersByToken map[string]*mcmodel.User
}

func newHubTestUserStor() *hubTestUserStor {
	return &hubTestUserStor{
		usersByToken: map[string]*mcmodel.User{
			"test-token":  {ID: 123, Name: "Test User"},
			"query-token": {ID: 456, Name: "Query User"},
		},
	}
}

func (s *hubTestUserStor) CreateUser(user *mcmodel.User) (*mcmodel.User, error) {
	return nil, errors.New("not implemented")
}

func (s *hubTestUserStor) GetUsersWithGlobusAccount() ([]mcmodel.User, error) {
	return nil, errors.New("not implemented")
}

func (s *hubTestUserStor) GetUserBySlug(slug string) (*mcmodel.User, error) {
	return nil, errors.New("not implemented")
}

func (s *hubTestUserStor) GetUserByEmail(email string) (*mcmodel.User, error) {
	return nil, errors.New("not implemented")
}

func (s *hubTestUserStor) GetUserByAPIToken(apiToken string) (*mcmodel.User, error) {
	user, ok := s.usersByToken[apiToken]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

type hubTestProjectStor struct {
	projects map[int]*mcmodel.Project
}

func newHubTestProjectStor() *hubTestProjectStor {
	return &hubTestProjectStor{
		projects: make(map[int]*mcmodel.Project),
	}
}

func (s *hubTestProjectStor) CreateProject(project *mcmodel.Project) (*mcmodel.Project, error) {
	return nil, errors.New("not implemented")
}

func (s *hubTestProjectStor) GetProjectByID(projectID int) (*mcmodel.Project, error) {
	project, ok := s.projects[projectID]
	if !ok {
		return nil, errors.New("project not found")
	}
	return project, nil
}

func (s *hubTestProjectStor) GetProjectBySlug(slug string) (*mcmodel.Project, error) {
	return nil, errors.New("not implemented")
}

func (s *hubTestProjectStor) GetProjectsForUser(userID int) ([]mcmodel.Project, error) {
	return nil, errors.New("not implemented")
}

func (s *hubTestProjectStor) UpdateProjectSizeAndFileCount(projectID int, size int64, fileCount int) error {
	return errors.New("not implemented")
}

func (s *hubTestProjectStor) UpdateProjectDirectoryCount(projectID int, directoryCount int) error {
	return errors.New("not implemented")
}

func (s *hubTestProjectStor) UserCanAccessProject(userID, projectID int) bool {
	return true
}

func (s *hubTestProjectStor) AddMemberToProject(project *mcmodel.Project, user *mcmodel.User) error {
	return errors.New("not implemented")
}

func (s *hubTestProjectStor) AddAdminToProject(project *mcmodel.Project, user *mcmodel.User) error {
	return errors.New("not implemented")
}

type hubTestRemoteClientStor struct {
	clientsByClientID map[string]*mcmodel.RemoteClient
	createdCount      int
}

func newHubTestRemoteClientStor() *hubTestRemoteClientStor {
	return &hubTestRemoteClientStor{
		clientsByClientID: make(map[string]*mcmodel.RemoteClient),
	}
}

func (s *hubTestRemoteClientStor) CreateRemoteClient(remoteClient *mcmodel.RemoteClient) (*mcmodel.RemoteClient, error) {
	s.createdCount++
	if remoteClient.ID == 0 {
		remoteClient.ID = s.createdCount
	}
	s.clientsByClientID[remoteClient.ClientID] = remoteClient
	return remoteClient, nil
}

func (s *hubTestRemoteClientStor) GetRemoteClientByClientID(clientID string) (*mcmodel.RemoteClient, error) {
	client, ok := s.clientsByClientID[clientID]
	if !ok {
		return nil, errors.New("remote client not found")
	}
	return client, nil
}
