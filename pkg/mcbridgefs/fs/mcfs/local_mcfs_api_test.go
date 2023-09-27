package mcfs

import (
	"testing"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/stretchr/testify/require"
)

func TestMCApi_Create(t *testing.T) {
	t.Skip("Skipping TestMCApi_Create")
	//files := []mcmodel.File{
	//	{ID: 456, Path: "/data", ProjectID: 123, MimeType: "directory"},
	//}
	//transferRequests := []mcmodel.TransferRequest{
	//	{ID: 234, ProjectID: 123, OwnerID: 301},
	//}
	knownFilesTracker := NewKnownFilesTracker()
	_, stors := newTestStor(t, "", "/tmp/mcfs")
	mcapi := NewLocalMCFSApi(stors, knownFilesTracker)

	var tests = []struct {
		name          string
		path          string
		f             *mcmodel.File
		expectedError error
	}{
		{
			name: "Test added file in knownFilesTracker",
			path: "/123/301/data/file.txt",
			f:    &mcmodel.File{ID: 123, Name: "file.txt"},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			createdFile, err := mcapi.Create(test.path)
			if test.expectedError != nil {
				require.Equal(t, test.expectedError, err)
			} else {
				require.Nil(t, err)
				require.NotNil(t, createdFile)
				f := knownFilesTracker.GetFile(test.path)
				require.NotNil(t, f)
				require.Equal(t, createdFile.ID, f.ID)
			}
		})
	}
}

func TestMCApi_Open(t *testing.T) {

}

func TestMCApi_GetRealPath(t *testing.T) {

}

func TestMCApi_Lookup(t *testing.T) {

}

func TestMCApi_Mkdir(t *testing.T) {

}

func TestMCApi_Readdir(t *testing.T) {

}

func TestMCApi_Release(t *testing.T) {

}
