package wserv

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

func TestAlreadyUploadedCreatesLinkedMCFTRecordWithChecksum(t *testing.T) {
	root := t.TempDir()

	const (
		projectID = 123
		ownerID   = 456
		checksum  = "5d41402abc4b2a76b9719d911017c592"
	)

	existing := &mcmodel.File{
		ID:           10,
		UUID:         "file-abcd-0000-0000-000000000001",
		ProjectID:    999,
		Name:         "canonical.dat",
		DirectoryID:  88,
		Size:         42,
		Checksum:     checksum,
		MimeType:     "application/octet-stream",
		Current:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		UploadSource: "original",
	}

	mustCreateUnderlyingFile(t, root, existing)

	fileStor := newFakeFileStor(root)
	fileStor.matchingByChecksum = existing
	fileStor.dirs["/target"] = &mcmodel.File{
		ID:        20,
		UUID:      "dir-abcd-0000-0000-000000000020",
		ProjectID: projectID,
		Name:      "target",
		Path:      "/target",
		MimeType:  "directory",
		Current:   true,
	}

	cc := newTestClientConnection(root, fileStor)
	cc.User.ID = ownerID

	ok, created := cc.alreadyUploaded(projectID, "/target/duplicate.dat", checksum)
	if !ok {
		t.Fatalf("alreadyUploaded returned false, want true")
	}
	if created == nil {
		t.Fatalf("created file is nil")
	}

	if created.Checksum != checksum {
		t.Fatalf("created checksum = %q, want %q", created.Checksum, checksum)
	}
	if created.Size != existing.Size {
		t.Fatalf("created size = %d, want %d", created.Size, existing.Size)
	}
	if created.UsesUUID != existing.UUIDForUses() {
		t.Fatalf("created UsesUUID = %q, want %q", created.UsesUUID, existing.UUIDForUses())
	}
	if created.UsesID != existing.IDForUses() {
		t.Fatalf("created UsesID = %d, want %d", created.UsesID, existing.IDForUses())
	}
	if created.UploadSource != "MCFT" {
		t.Fatalf("created UploadSource = %q, want MCFT", created.UploadSource)
	}
	if !created.Current {
		t.Fatalf("created file was not marked current")
	}
	if fileStor.createdCount != 1 {
		t.Fatalf("createdCount = %d, want 1", fileStor.createdCount)
	}
}

func TestAlreadyUploadedSamePathWithMatchingChecksumDoesNotCreateNewRecord(t *testing.T) {
	root := t.TempDir()

	const (
		projectID = 123
		ownerID   = 456
		checksum  = "5d41402abc4b2a76b9719d911017c592"
		filePath  = "/target/existing.dat"
	)

	existing := &mcmodel.File{
		ID:          10,
		UUID:        "file-abcd-0000-0000-000000000001",
		ProjectID:   projectID,
		Name:        "existing.dat",
		DirectoryID: 88,
		Size:        42,
		Checksum:    checksum,
		MimeType:    "application/octet-stream",
		Current:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mustCreateUnderlyingFile(t, root, existing)

	fileStor := newFakeFileStor(root)
	fileStor.matchingByChecksum = existing
	fileStor.filesByPath[filePath] = existing

	cc := newTestClientConnection(root, fileStor)
	cc.User.ID = ownerID

	ok, returned := cc.alreadyUploaded(projectID, filePath, checksum)
	if !ok {
		t.Fatalf("alreadyUploaded returned false, want true")
	}
	if returned.ID != existing.ID {
		t.Fatalf("returned file ID = %d, want existing ID %d", returned.ID, existing.ID)
	}
	if fileStor.createdCount != 0 {
		t.Fatalf("createdCount = %d, want 0", fileStor.createdCount)
	}
}

func TestAlreadyUploadedMissingUnderlyingFileFallsBackToUpload(t *testing.T) {
	root := t.TempDir()

	const (
		projectID = 123
		ownerID   = 456
		checksum  = "5d41402abc4b2a76b9719d911017c592"
	)

	existingMissingOnDisk := &mcmodel.File{
		ID:          10,
		UUID:        "file-abcd-0000-0000-000000000001",
		ProjectID:   projectID,
		Name:        "missing.dat",
		DirectoryID: 88,
		Size:        42,
		Checksum:    checksum,
		MimeType:    "application/octet-stream",
		Current:     true,
	}

	fileStor := newFakeFileStor(root)
	fileStor.matchingByChecksum = existingMissingOnDisk

	cc := newTestClientConnection(root, fileStor)
	cc.User.ID = ownerID

	ok, created := cc.alreadyUploaded(projectID, "/target/duplicate.dat", checksum)
	if ok {
		t.Fatalf("alreadyUploaded returned true for missing underlying file")
	}
	if created != nil {
		t.Fatalf("created = %+v, want nil", created)
	}
	if fileStor.createdCount != 0 {
		t.Fatalf("createdCount = %d, want 0", fileStor.createdCount)
	}
}

func TestHandleTransferInitDuplicateGoCLIAlreadyUploaded(t *testing.T) {
	root := t.TempDir()

	const (
		projectID  = 123
		ownerID    = 456
		transferID = "transfer-duplicate-gocli"
		checksum   = "5d41402abc4b2a76b9719d911017c592"
	)

	existing := &mcmodel.File{
		ID:          10,
		UUID:        "file-abcd-0000-0000-000000000001",
		ProjectID:   projectID,
		Name:        "canonical.dat",
		DirectoryID: 88,
		Size:        42,
		Checksum:    checksum,
		MimeType:    "application/octet-stream",
		Current:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	mustCreateUnderlyingFile(t, root, existing)

	fileStor := newFakeFileStor(root)
	fileStor.matchingByChecksum = existing
	fileStor.dirs["/target"] = &mcmodel.File{
		ID:        20,
		UUID:      "dir-abcd-0000-0000-000000000020",
		ProjectID: projectID,
		Name:      "target",
		Path:      "/target",
		MimeType:  "directory",
		Current:   true,
	}

	cc := newTestClientConnection(root, fileStor)
	cc.Type = "gocli"
	cc.User.ID = ownerID
	cc.Hub.ProjectStor.(*fakeProjectStor).allowed = true

	cc.handleTransferInit(validTransferInitMessage(transferID, projectID, checksum))

	msg := readSentMessage(t, cc)
	if msg.Command != MsgTransferAlreadyUploaded {
		t.Fatalf("command = %q, want %q", msg.Command, MsgTransferAlreadyUploaded)
	}

	payload := requirePayloadMap(t, msg)
	if payload["file_checksum"] != checksum {
		t.Fatalf("file_checksum = %q, want %q", payload["file_checksum"], checksum)
	}
	if fileStor.createdCount != 1 {
		t.Fatalf("createdCount = %d, want 1 linked record", fileStor.createdCount)
	}
	if cc.Hub.RemoteClientTransferStor.(*fakeRemoteClientTransferStor).createdCount != 0 {
		t.Fatalf("remote transfer was created for duplicate upload")
	}
}

func TestHandleTransferInitDuplicateLegacyCLIRejects(t *testing.T) {
	root := t.TempDir()

	const (
		projectID  = 123
		ownerID    = 456
		transferID = "transfer-duplicate-cli"
		checksum   = "5d41402abc4b2a76b9719d911017c592"
	)

	existing := &mcmodel.File{
		ID:          10,
		UUID:        "file-abcd-0000-0000-000000000001",
		ProjectID:   projectID,
		Name:        "canonical.dat",
		DirectoryID: 88,
		Size:        42,
		Checksum:    checksum,
		MimeType:    "application/octet-stream",
		Current:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	mustCreateUnderlyingFile(t, root, existing)

	fileStor := newFakeFileStor(root)
	fileStor.matchingByChecksum = existing
	fileStor.dirs["/target"] = &mcmodel.File{
		ID:        20,
		UUID:      "dir-abcd-0000-0000-000000000020",
		ProjectID: projectID,
		Name:      "target",
		Path:      "/target",
		MimeType:  "directory",
		Current:   true,
	}

	cc := newTestClientConnection(root, fileStor)
	cc.Type = "cli"
	cc.User.ID = ownerID
	cc.Hub.ProjectStor.(*fakeProjectStor).allowed = true

	cc.handleTransferInit(validTransferInitMessage(transferID, projectID, checksum))

	msg := readSentMessage(t, cc)
	if msg.Command != MsgTransferReject {
		t.Fatalf("command = %q, want %q", msg.Command, MsgTransferReject)
	}

	payload := requirePayloadMap(t, msg)
	if payload["reason"] != "file already uploaded" {
		t.Fatalf("reason = %q, want %q", payload["reason"], "file already uploaded")
	}
	if cc.Hub.RemoteClientTransferStor.(*fakeRemoteClientTransferStor).createdCount != 0 {
		t.Fatalf("remote transfer was created for duplicate upload")
	}
}

func TestHandleTransferInitRejectsDuplicateTransferID(t *testing.T) {
	root := t.TempDir()

	const (
		projectID  = 123
		transferID = "transfer-already-exists"
		checksum   = "5d41402abc4b2a76b9719d911017c592"
	)

	fileStor := newFakeFileStor(root)
	cc := newTestClientConnection(root, fileStor)
	cc.Hub.ProjectStor.(*fakeProjectStor).allowed = true
	cc.Hub.RemoteClientTransferStor.(*fakeRemoteClientTransferStor).transfers[transferID] = &mcmodel.RemoteClientTransfer{
		ID:         99,
		TransferID: transferID,
	}

	cc.handleTransferInit(validTransferInitMessage(transferID, projectID, checksum))

	msg := readSentMessage(t, cc)
	if msg.Command != MsgTransferReject {
		t.Fatalf("command = %q, want %q", msg.Command, MsgTransferReject)
	}

	payload := requirePayloadMap(t, msg)
	if payload["reason"] != "transfer already exists" {
		t.Fatalf("reason = %q, want %q", payload["reason"], "transfer already exists")
	}
}

func TestHandleTransferInitRejectsNoProjectAccess(t *testing.T) {
	root := t.TempDir()

	const (
		projectID  = 123
		transferID = "transfer-no-access"
		checksum   = "5d41402abc4b2a76b9719d911017c592"
	)

	fileStor := newFakeFileStor(root)
	cc := newTestClientConnection(root, fileStor)
	cc.Hub.ProjectStor.(*fakeProjectStor).allowed = false

	cc.handleTransferInit(validTransferInitMessage(transferID, projectID, checksum))

	msg := readSentMessage(t, cc)
	if msg.Command != MsgTransferReject {
		t.Fatalf("command = %q, want %q", msg.Command, MsgTransferReject)
	}

	payload := requirePayloadMap(t, msg)
	if payload["reason"] != "no access to project" {
		t.Fatalf("reason = %q, want %q", payload["reason"], "no access to project")
	}
}

func TestHandleTransferInitRejectsInvalidSize(t *testing.T) {
	root := t.TempDir()

	const (
		projectID  = 123
		transferID = "transfer-invalid-size"
		checksum   = "5d41402abc4b2a76b9719d911017c592"
	)

	fileStor := newFakeFileStor(root)
	cc := newTestClientConnection(root, fileStor)
	cc.Hub.ProjectStor.(*fakeProjectStor).allowed = true

	msg := validTransferInitMessage(transferID, projectID, checksum)
	msg.Payload.(map[string]interface{})["file_size"] = float64(0)

	cc.handleTransferInit(msg)

	sent := readSentMessage(t, cc)
	if sent.Command != MsgTransferReject {
		t.Fatalf("command = %q, want %q", sent.Command, MsgTransferReject)
	}

	payload := requirePayloadMap(t, sent)
	if payload["reason"] != "invalid parameters" {
		t.Fatalf("reason = %q, want %q", payload["reason"], "invalid parameters")
	}
}

func TestHandleTransferInitSuccessfulCreatesRemoteTransferAndActiveTransfer(t *testing.T) {
	root := t.TempDir()

	const (
		projectID  = 123
		transferID = "transfer-success"
		checksum   = "5d41402abc4b2a76b9719d911017c592"
	)

	fileStor := newFakeFileStor(root)
	fileStor.dirs["/target"] = &mcmodel.File{
		ID:        20,
		UUID:      "dir-abcd-0000-0000-000000000020",
		ProjectID: projectID,
		Name:      "target",
		Path:      "/target",
		MimeType:  "directory",
		Current:   true,
	}

	cc := newTestClientConnection(root, fileStor)
	cc.Hub.ProjectStor.(*fakeProjectStor).allowed = true

	msg := validTransferInitMessage(transferID, projectID, checksum)
	msg.Payload.(map[string]interface{})["file_size"] = float64(11)
	msg.Payload.(map[string]interface{})["chunk_size"] = float64(5)

	cc.handleTransferInit(msg)

	sent := readSentMessage(t, cc)
	if sent.Command != MsgTransferAccept {
		t.Fatalf("command = %q, want %q", sent.Command, MsgTransferAccept)
	}

	payload := requirePayloadMap(t, sent)
	if payload["expected_chunks"] != 3 {
		t.Fatalf("expected_chunks = %v, want 3", payload["expected_chunks"])
	}

	transferStor := cc.Hub.RemoteClientTransferStor.(*fakeRemoteClientTransferStor)
	if transferStor.createdCount != 1 {
		t.Fatalf("created remote transfers = %d, want 1", transferStor.createdCount)
	}

	remoteTransfer := transferStor.transfers[transferID]
	if remoteTransfer == nil {
		t.Fatalf("remote transfer %q was not stored", transferID)
	}
	if remoteTransfer.ExpectedChecksum != checksum {
		t.Fatalf("remote transfer checksum = %q, want %q", remoteTransfer.ExpectedChecksum, checksum)
	}
	if remoteTransfer.ExpectedSize != 11 {
		t.Fatalf("remote transfer expected size = %d, want 11", remoteTransfer.ExpectedSize)
	}
	if remoteTransfer.ChunkSize != 5 {
		t.Fatalf("remote transfer chunk size = %d, want 5", remoteTransfer.ChunkSize)
	}

	cc.transferMu.RLock()
	active := cc.activeTransfers[transferID]
	cc.transferMu.RUnlock()
	if active == nil {
		t.Fatalf("active transfer was not registered")
	}
	if active.ExpectedSize != 11 {
		t.Fatalf("active expected size = %d, want 11", active.ExpectedSize)
	}
	if active.ChunkSize != 5 {
		t.Fatalf("active chunk size = %d, want 5", active.ChunkSize)
	}
}

func TestHandleTransferInitMalformedPayloadsReject(t *testing.T) {
	tests := []struct {
		name           string
		mutate         func(Message) Message
		wantTransferID string
	}{
		{
			name: "nil payload",
			mutate: func(msg Message) Message {
				msg.Payload = nil
				return msg
			},
			wantTransferID: "",
		},
		{
			name: "payload is not a map",
			mutate: func(msg Message) Message {
				msg.Payload = "not a map"
				return msg
			},
			wantTransferID: "",
		},
		{
			name: "missing transfer id",
			mutate: func(msg Message) Message {
				delete(msg.Payload.(map[string]interface{}), "transfer_id")
				return msg
			},
			wantTransferID: "",
		},
		{
			name: "empty transfer id",
			mutate: func(msg Message) Message {
				msg.Payload.(map[string]interface{})["transfer_id"] = "   "
				return msg
			},
			wantTransferID: "",
		},
		{
			name: "missing project path",
			mutate: func(msg Message) Message {
				delete(msg.Payload.(map[string]interface{}), "project_path")
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "empty project path",
			mutate: func(msg Message) Message {
				msg.Payload.(map[string]interface{})["project_path"] = "   "
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "missing file path",
			mutate: func(msg Message) Message {
				delete(msg.Payload.(map[string]interface{}), "file_path")
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "empty file path",
			mutate: func(msg Message) Message {
				msg.Payload.(map[string]interface{})["file_path"] = "   "
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "missing file size",
			mutate: func(msg Message) Message {
				delete(msg.Payload.(map[string]interface{}), "file_size")
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "zero file size",
			mutate: func(msg Message) Message {
				msg.Payload.(map[string]interface{})["file_size"] = float64(0)
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "negative file size",
			mutate: func(msg Message) Message {
				msg.Payload.(map[string]interface{})["file_size"] = float64(-1)
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "file size has wrong type",
			mutate: func(msg Message) Message {
				msg.Payload.(map[string]interface{})["file_size"] = "11"
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "missing chunk size",
			mutate: func(msg Message) Message {
				delete(msg.Payload.(map[string]interface{}), "chunk_size")
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "zero chunk size",
			mutate: func(msg Message) Message {
				msg.Payload.(map[string]interface{})["chunk_size"] = float64(0)
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "negative chunk size",
			mutate: func(msg Message) Message {
				msg.Payload.(map[string]interface{})["chunk_size"] = float64(-1)
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "missing project id",
			mutate: func(msg Message) Message {
				delete(msg.Payload.(map[string]interface{}), "project_id")
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "zero project id",
			mutate: func(msg Message) Message {
				msg.Payload.(map[string]interface{})["project_id"] = float64(0)
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "negative project id",
			mutate: func(msg Message) Message {
				msg.Payload.(map[string]interface{})["project_id"] = float64(-123)
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "missing checksum",
			mutate: func(msg Message) Message {
				delete(msg.Payload.(map[string]interface{}), "checksum")
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
		{
			name: "empty checksum",
			mutate: func(msg Message) Message {
				msg.Payload.(map[string]interface{})["checksum"] = "   "
				return msg
			},
			wantTransferID: "transfer-malformed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			cc := newTestClientConnection(root, newFakeFileStor(root))
			msg := tt.mutate(validTransferInitMessage("transfer-malformed", 123, "5d41402abc4b2a76b9719d911017c592"))

			cc.handleTransferInit(msg)

			sent := readSentMessage(t, cc)
			if sent.Command != MsgTransferReject {
				t.Fatalf("command = %q, want %q", sent.Command, MsgTransferReject)
			}

			payload := requirePayloadMap(t, sent)
			if payload["reason"] != "invalid parameters" {
				t.Fatalf("reason = %q, want %q", payload["reason"], "invalid parameters")
			}
			if payload["transfer_id"] != tt.wantTransferID {
				t.Fatalf("transfer_id = %q, want %q", payload["transfer_id"], tt.wantTransferID)
			}

			if cc.Hub.RemoteClientTransferStor.(*fakeRemoteClientTransferStor).createdCount != 0 {
				t.Fatalf("remote transfer was created for malformed payload")
			}
			if cc.Hub.FileStor.(*fakeFileStor).createdCount != 0 {
				t.Fatalf("file was created for malformed payload")
			}
		})
	}
}

func validTransferInitMessage(transferID string, projectID int, checksum string) Message {
	return Message{
		Command:  MsgTransferInit,
		ID:       "message-id",
		ClientID: "client-id",
		Payload: map[string]interface{}{
			"transfer_id":  transferID,
			"project_path": "/target/upload.dat",
			"file_path":    "/tmp/upload.dat",
			"file_size":    float64(10),
			"chunk_size":   float64(5),
			"project_id":   float64(projectID),
			"checksum":     checksum,
		},
	}
}

func newTestClientConnection(root string, fileStor *fakeFileStor) *ClientConnection {
	return &ClientConnection{
		ID:              "client-id",
		Type:            "gocli",
		User:            &mcmodel.User{ID: 1},
		RemoteClient:    &mcmodel.RemoteClient{ID: 77},
		Send:            make(chan Message, 20),
		activeTransfers: make(map[string]*FileTransfer),
		Hub: &Hub{
			FileStor:                 fileStor,
			ProjectStor:              &fakeProjectStor{allowed: true},
			RemoteClientTransferStor: newFakeRemoteClientTransferStor(),
			ConversionStor:           &fakeConversionStor{},
		},
	}
}

func readSentMessage(t *testing.T, cc *ClientConnection) Message {
	t.Helper()

	select {
	case msg := <-cc.Send:
		return msg
	default:
		t.Fatalf("no message sent")
		return Message{}
	}
}

func requirePayloadMap(t *testing.T, msg Message) map[string]interface{} {
	t.Helper()

	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("payload = %#v, want map[string]interface{}", msg.Payload)
	}
	return payload
}

func mustCreateUnderlyingFile(t *testing.T, root string, file *mcmodel.File) {
	t.Helper()

	if err := os.MkdirAll(file.ToUnderlyingDirPath(root), 0755); err != nil {
		t.Fatalf("mkdir underlying path: %v", err)
	}

	if err := os.WriteFile(file.ToUnderlyingFilePath(root), []byte("content"), 0644); err != nil {
		t.Fatalf("write underlying file: %v", err)
	}
}

type fakeProjectStor struct {
	allowed bool
}

func (s *fakeProjectStor) CreateProject(project *mcmodel.Project) (*mcmodel.Project, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeProjectStor) GetProjectByID(projectID int) (*mcmodel.Project, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeProjectStor) GetProjectBySlug(slug string) (*mcmodel.Project, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeProjectStor) GetProjectsForUser(userID int) ([]mcmodel.Project, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeProjectStor) UpdateProjectSizeAndFileCount(projectID int, size int64, fileCount int) error {
	return errors.New("not implemented")
}

func (s *fakeProjectStor) UpdateProjectDirectoryCount(projectID int, directoryCount int) error {
	return errors.New("not implemented")
}

func (s *fakeProjectStor) UserCanAccessProject(userID, projectID int) bool {
	return s.allowed
}

func (s *fakeProjectStor) AddMemberToProject(project *mcmodel.Project, user *mcmodel.User) error {
	return errors.New("not implemented")
}

func (s *fakeProjectStor) AddAdminToProject(project *mcmodel.Project, user *mcmodel.User) error {
	return errors.New("not implemented")
}

type fakeRemoteClientTransferStor struct {
	transfers    map[string]*mcmodel.RemoteClientTransfer
	createdCount int
}

func newFakeRemoteClientTransferStor() *fakeRemoteClientTransferStor {
	return &fakeRemoteClientTransferStor{
		transfers: make(map[string]*mcmodel.RemoteClientTransfer),
	}
}

func (s *fakeRemoteClientTransferStor) CreateRemoteClientTransfer(clientTransfer *mcmodel.RemoteClientTransfer) (*mcmodel.RemoteClientTransfer, error) {
	s.createdCount++
	if clientTransfer.ID == 0 {
		clientTransfer.ID = s.createdCount
	}
	s.transfers[clientTransfer.TransferID] = clientTransfer
	return clientTransfer, nil
}

func (s *fakeRemoteClientTransferStor) GetRemoteClientTransferByTransferID(transferID string) (*mcmodel.RemoteClientTransfer, error) {
	transfer, ok := s.transfers[transferID]
	if !ok {
		return nil, errors.New("not found")
	}
	return transfer, nil
}

func (s *fakeRemoteClientTransferStor) UpdateRemoteClientTransferState(UUID string, state string) (*mcmodel.RemoteClientTransfer, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeRemoteClientTransferStor) DeleteRemoteClientTransferByTransferID(transferID string) error {
	delete(s.transfers, transferID)
	return nil
}

func (s *fakeRemoteClientTransferStor) GetAllTransfersForRemoteClient(remoteClientID int) ([]mcmodel.RemoteClientTransfer, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeRemoteClientTransferStor) GetAllUploadTransfersForRemoteClient(remoteClientID int) ([]mcmodel.RemoteClientTransfer, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeRemoteClientTransferStor) GetAllDownloadTransfersForRemoteClient(remoteClientID int) ([]mcmodel.RemoteClientTransfer, error) {
	return nil, errors.New("not implemented")
}

type fakeFileStor struct {
	root string

	matchingByChecksum *mcmodel.File
	filesByPath        map[string]*mcmodel.File
	dirs               map[string]*mcmodel.File
	filesByID          map[int]*mcmodel.File

	nextID       int
	created      []*mcmodel.File
	createdCount int
}

func newFakeFileStor(root string) *fakeFileStor {
	return &fakeFileStor{
		root:        root,
		filesByPath: make(map[string]*mcmodel.File),
		dirs:        make(map[string]*mcmodel.File),
		filesByID:   make(map[int]*mcmodel.File),
		nextID:      100,
	}
}

func (s *fakeFileStor) Root() string {
	return s.root
}

func (s *fakeFileStor) FindMatchingFileByChecksum(checksum string) (*mcmodel.File, error) {
	if s.matchingByChecksum == nil || s.matchingByChecksum.Checksum != checksum {
		return nil, errors.New("not found")
	}
	return s.matchingByChecksum, nil
}

func (s *fakeFileStor) GetFileByPath(projectID int, path string) (*mcmodel.File, error) {
	file, ok := s.filesByPath[path]
	if !ok {
		return nil, errors.New("not found")
	}
	return file, nil
}

func (s *fakeFileStor) GetOrCreateDirPath(projectID, ownerID int, path string) (*mcmodel.File, error) {
	if dir, ok := s.dirs[path]; ok {
		return dir, nil
	}

	s.nextID++
	dir := &mcmodel.File{
		ID:        s.nextID,
		UUID:      "dir-abcd-0000-0000-000000000099",
		ProjectID: projectID,
		OwnerID:   ownerID,
		Name:      filepath.Base(path),
		Path:      path,
		MimeType:  "directory",
		Current:   true,
	}
	s.dirs[path] = dir
	return dir, nil
}

func (s *fakeFileStor) CreateFile(name string, projectID, directoryID, ownerID int, mimeType string) (*mcmodel.File, error) {
	s.nextID++
	file := &mcmodel.File{
		ID:          s.nextID,
		UUID:        "file-abcd-0000-0000-000000000099",
		ProjectID:   projectID,
		OwnerID:     ownerID,
		Name:        name,
		DirectoryID: directoryID,
		MimeType:    mimeType,
		Current:     false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	s.created = append(s.created, file)
	s.filesByID[file.ID] = file
	s.createdCount++
	return file, nil
}

func (s *fakeFileStor) UpdateFile(file, updates *mcmodel.File) (*mcmodel.File, error) {
	file.Size = updates.Size
	file.Checksum = updates.Checksum
	file.UsesUUID = updates.UsesUUID
	file.UsesID = updates.UsesID
	file.UploadSource = updates.UploadSource
	return file, nil
}

func (s *fakeFileStor) SetFileAsCurrent(file *mcmodel.File) (*mcmodel.File, error) {
	file.Current = true
	return file, nil
}

func (s *fakeFileStor) GetFileByID(fileID int) (*mcmodel.File, error) {
	file, ok := s.filesByID[fileID]
	if !ok {
		return nil, errors.New("not found")
	}
	return file, nil
}

func (s *fakeFileStor) GetFileByUUID(fileUUID string) (*mcmodel.File, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeFileStor) UpdateMetadataForFileAndProject(file *mcmodel.File, checksum string, totalBytes int64) error {
	return errors.New("not implemented")
}

func (s *fakeFileStor) SetUsesToNull(file *mcmodel.File) (*mcmodel.File, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeFileStor) GetDirByPath(projectID int, path string) (*mcmodel.File, error) {
	dir, ok := s.dirs[path]
	if !ok {
		return nil, errors.New("not found")
	}
	return dir, nil
}

func (s *fakeFileStor) CreateDirectory(parentDirID, projectID, ownerID int, path, name string) (*mcmodel.File, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeFileStor) CreateDirIfNotExists(parentDirID int, path, name string, projectID, ownerID int) (*mcmodel.File, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeFileStor) ListDirectoryByPath(projectID int, path string) ([]mcmodel.File, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeFileStor) UpdateFileUses(file *mcmodel.File, uuid string, fileID int) error {
	return errors.New("not implemented")
}

func (s *fakeFileStor) PointAtExistingIfExists(file *mcmodel.File) (bool, error) {
	return false, errors.New("not implemented")
}

func (s *fakeFileStor) DoneWritingToFile(file *mcmodel.File, checksum string, size int64, conversionStore stor.ConversionStor) (bool, error) {
	return false, errors.New("not implemented")
}

func (s *fakeFileStor) GetMatchingFileInDirectory(directoryID int, checksum string, name string) (*mcmodel.File, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeFileStor) SetFileHealthMissing(file *mcmodel.File, determinedBy string, source string) (*mcmodel.File, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeFileStor) SetFileHealthFixed(file *mcmodel.File, fixedBy string, source string) (*mcmodel.File, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeFileStor) FindMatchingFileByChecksumAndPath(projectID int, filePath string, checksum string) (*mcmodel.File, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeFileStor) DeleteFileByID(ID int) error {
	return errors.New("not implemented")
}

func (s *fakeFileStor) ListFileVersionsForName(projectID int, directoryID int, name string) ([]mcmodel.File, error) {
	return nil, errors.New("not implemented")
}

type fakeConversionStor struct{}

func (s *fakeConversionStor) AddFileToConvert(file *mcmodel.File) (*mcmodel.Conversion, error) {
	return &mcmodel.Conversion{}, nil
}
