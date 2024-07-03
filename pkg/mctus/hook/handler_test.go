package hook

import (
	"reflect"
	"sync"
	"testing"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/tus/tusd/v2/pkg/hooks"
	"gorm.io/gorm"
)

func TestMCHookHandler_InvokeHook(t *testing.T) {
	type fields struct {
		projectStor         stor.ProjectStor
		fileStor            stor.FileStor
		userStor            stor.UserStor
		dirCacheByID        map[int]*mcmodel.File
		userCacheByAPIKey   map[string]*mcmodel.User
		userIDToProjectList map[int][]int
		accessCount         int
		mu                  sync.Mutex
	}
	type args struct {
		req hooks.HookRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantRes hooks.HookResponse
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &MCHookHandler{
				projectStor:         tt.fields.projectStor,
				fileStor:            tt.fields.fileStor,
				userStor:            tt.fields.userStor,
				dirCacheByID:        tt.fields.dirCacheByID,
				userCacheByAPIKey:   tt.fields.userCacheByAPIKey,
				userIDToProjectList: tt.fields.userIDToProjectList,
				accessCount:         tt.fields.accessCount,
				mu:                  tt.fields.mu,
			}
			gotRes, err := h.InvokeHook(tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("InvokeHook() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("InvokeHook() gotRes = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func TestMCHookHandler_Setup(t *testing.T) {
	type fields struct {
		projectStor         stor.ProjectStor
		fileStor            stor.FileStor
		userStor            stor.UserStor
		dirCacheByID        map[int]*mcmodel.File
		userCacheByAPIKey   map[string]*mcmodel.User
		userIDToProjectList map[int][]int
		accessCount         int
		mu                  sync.Mutex
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &MCHookHandler{
				projectStor:         tt.fields.projectStor,
				fileStor:            tt.fields.fileStor,
				userStor:            tt.fields.userStor,
				dirCacheByID:        tt.fields.dirCacheByID,
				userCacheByAPIKey:   tt.fields.userCacheByAPIKey,
				userIDToProjectList: tt.fields.userIDToProjectList,
				accessCount:         tt.fields.accessCount,
				mu:                  tt.fields.mu,
			}
			if err := h.Setup(); (err != nil) != tt.wantErr {
				t.Errorf("Setup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMCHookHandler_getFileByID(t *testing.T) {
	type fields struct {
		projectStor         stor.ProjectStor
		fileStor            stor.FileStor
		userStor            stor.UserStor
		dirCacheByID        map[int]*mcmodel.File
		userCacheByAPIKey   map[string]*mcmodel.User
		userIDToProjectList map[int][]int
		accessCount         int
		mu                  sync.Mutex
	}
	type args struct {
		id int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *mcmodel.File
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &MCHookHandler{
				projectStor:         tt.fields.projectStor,
				fileStor:            tt.fields.fileStor,
				userStor:            tt.fields.userStor,
				dirCacheByID:        tt.fields.dirCacheByID,
				userCacheByAPIKey:   tt.fields.userCacheByAPIKey,
				userIDToProjectList: tt.fields.userIDToProjectList,
				accessCount:         tt.fields.accessCount,
				mu:                  tt.fields.mu,
			}
			got, err := h.getFileByID(tt.args.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("getFileByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getFileByID() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMCHookHandler_getUserByAPIToken(t *testing.T) {
	type fields struct {
		projectStor         stor.ProjectStor
		fileStor            stor.FileStor
		userStor            stor.UserStor
		dirCacheByID        map[int]*mcmodel.File
		userCacheByAPIKey   map[string]*mcmodel.User
		userIDToProjectList map[int][]int
		accessCount         int
		mu                  sync.Mutex
	}
	type args struct {
		apiToken string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *mcmodel.User
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &MCHookHandler{
				projectStor:         tt.fields.projectStor,
				fileStor:            tt.fields.fileStor,
				userStor:            tt.fields.userStor,
				dirCacheByID:        tt.fields.dirCacheByID,
				userCacheByAPIKey:   tt.fields.userCacheByAPIKey,
				userIDToProjectList: tt.fields.userIDToProjectList,
				accessCount:         tt.fields.accessCount,
				mu:                  tt.fields.mu,
			}
			got, err := h.getUserByAPIToken(tt.args.apiToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("getUserByAPIToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getUserByAPIToken() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMCHookHandler_userCanAccessProject(t *testing.T) {
	type fields struct {
		projectStor         stor.ProjectStor
		fileStor            stor.FileStor
		userStor            stor.UserStor
		dirCacheByID        map[int]*mcmodel.File
		userCacheByAPIKey   map[string]*mcmodel.User
		userIDToProjectList map[int][]int
		accessCount         int
		mu                  sync.Mutex
	}
	type args struct {
		userID    int
		projectID int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &MCHookHandler{
				projectStor:         tt.fields.projectStor,
				fileStor:            tt.fields.fileStor,
				userStor:            tt.fields.userStor,
				dirCacheByID:        tt.fields.dirCacheByID,
				userCacheByAPIKey:   tt.fields.userCacheByAPIKey,
				userIDToProjectList: tt.fields.userIDToProjectList,
				accessCount:         tt.fields.accessCount,
				mu:                  tt.fields.mu,
			}
			if got := h.userCanAccessProject(tt.args.userID, tt.args.projectID); got != tt.want {
				t.Errorf("userCanAccessProject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewMCHookHandler(t *testing.T) {
	type args struct {
		db *gorm.DB
	}
	tests := []struct {
		name string
		args args
		want *MCHookHandler
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewMCHookHandler(tt.args.db); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewMCHookHandler() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_rejectRequest(t *testing.T) {
	type args struct {
		reason string
	}
	tests := []struct {
		name    string
		args    args
		wantRes hooks.HookResponse
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotRes := rejectRequest(tt.args.reason); !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("rejectRequest() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func Test_reqHasMetadata(t *testing.T) {
	type args struct {
		req hooks.HookRequest
		key string
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := reqHasMetadata(tt.args.req, tt.args.key)
			if got != tt.want {
				t.Errorf("reqHasMetadata() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("reqHasMetadata() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
