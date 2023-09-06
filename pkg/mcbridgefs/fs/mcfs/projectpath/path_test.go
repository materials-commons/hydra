package projectpath

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectPath(t *testing.T) {
	var tests = []struct {
		path                string
		expectedProjectUUID string
		expectedUserUUID    string
		expectedProjectPath string
	}{
		{
			path:                "/project1/user1/dir1",
			expectedProjectUUID: "project1",
			expectedUserUUID:    "user1",
			expectedProjectPath: "/dir1",
		},
		{
			path:                "/project1/user1/dir1/dir2/dir3",
			expectedProjectUUID: "project1",
			expectedUserUUID:    "user1",
			expectedProjectPath: "/dir1/dir2/dir3",
		},
		{
			path:                "/project1/user1/dir1/../dir2",
			expectedProjectUUID: "project1",
			expectedUserUUID:    "user1",
			expectedProjectPath: "/dir2",
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			p := NewProjectPath(test.path)
			require.Equal(t, test.expectedProjectUUID, p.ProjectUUID)
			require.Equal(t, test.expectedUserUUID, p.UserUUID)
			require.Equal(t, test.expectedProjectPath, p.ProjectPath)
			require.Equal(t, filepath.Clean(test.path), p.FullPath)
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
		{name: "simple join", path: "/project1/user/dir1", join: "/dir2", expected: "/dir1/dir2"},
		{name: "relative path join", path: "/project1/user/dir1/dir2/dir3", join: "/dir4/../dir5", expected: "/dir1/dir2/dir3/dir5"},
		{name: "relative path and project join", path: "/project1/user/dir1/../dir2", join: "/dir3/../dir4", expected: "/dir2/dir4"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			p := NewProjectPath(test.path)
			joined := p.Join(test.join)
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
		{name: "simple join", path: "/project1/user1/dir1", join: "/dir2", expected: "/dir1/dir2"},
		{name: "relative path join", path: "/project1/user1/dir1/dir2/dir3", join: "/dir4/../dir5", expected: "/dir1/dir2/dir3/dir5"},
		{name: "relative path and project join", path: "/project1/user1/dir1/../dir2", join: "/dir3/../dir4", expected: "/dir2/dir4"},
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
		{path: "/project1/user/dir1", expected: "/dir1"},
		{path: "/project1/user/dir1/dir2/dir3", expected: "/dir1/dir2/dir3"},
		{path: "/project1/user/dir1/../dir2", expected: "/dir2"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			require.Equal(t, test.expected, ToProjectPath(test.path))
		})
	}
}
