package mcfs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
