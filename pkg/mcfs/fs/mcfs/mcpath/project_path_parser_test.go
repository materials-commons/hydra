package mcpath

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectPath(t *testing.T) {
	var tests = []struct {
		path                 string
		expectedProjectID    int
		expectedUserID       int
		expectedProjectPath  string
		expectedTransferBase string
		expectPathType       PathType
	}{
		{
			path:                 "/25/301/dir1",
			expectedProjectID:    25,
			expectedUserID:       301,
			expectedProjectPath:  "/dir1",
			expectedTransferBase: "/25/301",
			expectPathType:       CompleteBasePath,
		},
		{
			path:                 "/25/301/dir1/dir2/dir3",
			expectedProjectID:    25,
			expectedUserID:       301,
			expectedProjectPath:  "/dir1/dir2/dir3",
			expectedTransferBase: "/25/301",
			expectPathType:       CompleteBasePath,
		},
		{
			path:                 "/25/301/dir1/../dir2",
			expectedProjectID:    25,
			expectedUserID:       301,
			expectedProjectPath:  "/dir2",
			expectedTransferBase: "/25/301",
			expectPathType:       CompleteBasePath,
		},
		{
			path:                 "/25/301",
			expectedProjectID:    25,
			expectedUserID:       301,
			expectedProjectPath:  "/",
			expectedTransferBase: "/25/301",
			expectPathType:       UserBasePath,
		},
		{
			path:                 "/25/301/",
			expectedProjectID:    25,
			expectedUserID:       301,
			expectedProjectPath:  "/",
			expectedTransferBase: "/25/301",
			expectPathType:       UserBasePath,
		},
		{
			path:                 "/25",
			expectedProjectID:    25,
			expectedUserID:       -1,
			expectedProjectPath:  "/",
			expectedTransferBase: "/",
			expectPathType:       ProjectBasePath,
		},
		{
			path:                 "/not-a-number",
			expectedProjectID:    -1,
			expectedUserID:       -1,
			expectedProjectPath:  "/",
			expectedTransferBase: "/",
			expectPathType:       BadIDPath,
		},
		{
			path:                 "/25/not-a-number",
			expectedProjectID:    25,
			expectedUserID:       -1,
			expectedProjectPath:  "/",
			expectedTransferBase: "/",
			expectPathType:       BadIDPath,
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			parser := NewProjectPathParser(nil)
			p, err := parser.Parse(test.path)
			if test.expectPathType == BadIDPath {
				require.Error(t, err)
			}
			require.Equal(t, test.expectedProjectID, p.ProjectID())
			require.Equal(t, test.expectedUserID, p.UserID())
			require.Equal(t, test.expectedProjectPath, p.ProjectPath())
			require.Equal(t, test.expectedTransferBase, p.TransferBase())
			require.Equal(t, test.expectPathType, p.PathType())
			require.Equal(t, filepath.Clean(test.path), p.FullPath())
		})
	}
}

func TestProjectPath_Join(t *testing.T) {
	var tests = []struct {
		name     string
		path     string
		join     string
		expected string
	}{
		{name: "simple join", path: "/25/301/dir1", join: "/dir2", expected: "/dir1/dir2"},
		{name: "relative path join", path: "/25/301/dir1/dir2/dir3", join: "/dir4/../dir5", expected: "/dir1/dir2/dir3/dir5"},
		{name: "relative path and project join", path: "/25/301/dir1/../dir2", join: "/dir3/../dir4", expected: "/dir2/dir4"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			parser := NewProjectPathParser(nil)
			p, err := parser.Parse(test.path)
			require.NoError(t, err)
			asProjPath := p.(*ProjectPath)
			joined := asProjPath.Join(test.join)
			require.Equal(t, test.expected, joined)
		})
	}
}

func TestJoin(t *testing.T) {
	var tests = []struct {
		name     string
		path     string
		join     string
		expected string
	}{
		{name: "simple join", path: "/25/301/dir1", join: "/dir2", expected: "/dir1/dir2"},
		{name: "relative path join", path: "/25/301/dir1/dir2/dir3", join: "/dir4/../dir5", expected: "/dir1/dir2/dir3/dir5"},
		{name: "relative path and project join", path: "/25/301/dir1/../dir2", join: "/dir3/../dir4", expected: "/dir2/dir4"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			joined := Join(test.path, test.join)
			require.Equal(t, test.expected, joined)
		})
	}
}

func TestToProjectPath(t *testing.T) {
	var tests = []struct {
		path     string
		expected string
	}{
		{path: "/25/301/dir1", expected: "/dir1"},
		{path: "/25/301/dir1/dir2/dir3", expected: "/dir1/dir2/dir3"},
		{path: "/25/301/dir1/../dir2", expected: "/dir2"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			require.Equal(t, test.expected, ToProjectPath(test.path))
		})
	}
}
