package mcfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListingTransferRequestProjects(t *testing.T) {
	var tests = []struct {
		name        string
		dir         string
		numEntries  int
		errExpected bool
	}{
		{name: "list projects", dir: "/tmp/mnt/mcfs", numEntries: 1, errExpected: false},
		{name: "project does not exist", dir: "/tmp/mnt/mcfs/2", numEntries: 0, errExpected: true},
	}

	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries, err := os.ReadDir(test.dir)
			if test.errExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, entries, test.numEntries)
				// test only returns one project, with id 1
				entry := entries[0]
				require.True(t, entry.IsDir())
				require.Equal(t, "1", entry.Name())
			}
		})
	}
}

func TestListingTransferRequestProjectUsers(t *testing.T) {
	// newTestCase creates a single project that has id 1, with a single user
	// with id 1

	var tests = []struct {
		name        string
		dir         string
		numEntries  int
		errExpected bool
	}{
		{name: "test existing user exists", dir: "/tmp/mnt/mcfs/1", numEntries: 1, errExpected: false},
		{name: "test user does not exist", dir: "/tmp/mnt/mcfs/2", numEntries: 0, errExpected: true},
	}

	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries, err := os.ReadDir(test.dir)
			if test.errExpected {
				require.Errorf(t, err, "Should have gotten an error when reading %s", test.dir)
			} else {
				require.NoError(t, err)
				require.Len(t, entries, 1)
				entry := entries[0]
				require.True(t, entry.IsDir())
				require.Equal(t, "1", entry.Name())
			}
		})
	}
}

func TestLookup(t *testing.T) {
	// newTestCase will create a single project id 1, with a single user with id 1.
	// Lookup will look for all parent paths. We are going to check that look up is
	// working by doing a stat on these items.
	var tests = []struct {
		name        string
		path        string
		errExpected bool
	}{
		{name: "Check that project 1 exists", path: "/tmp/mnt/mcfs/1", errExpected: false},
		{name: "Check that project 2 does not exist", path: "/tmp/mnt/mcfs/2", errExpected: true},
		{name: "Check that project 1/user 1 exists", path: "/tmp/mnt/mcfs/1/1", errExpected: false},
		{name: "Check that project 1/user 2 does not exist", path: "/tmp/mnt/mcfs/1/2", errExpected: true},
	}

	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			finfo, err := os.Stat(test.path)
			if test.errExpected {
				require.Errorf(t, err, "Expected err for path %s", test.path)
			} else {

				require.NoErrorf(t, err, "Expected no error for path %s, got %s", test.path, err)
				require.Truef(t, finfo.IsDir(), "Expected %s to be a dir", test.path)
			}
		})
	}
}

func TestMkdir(t *testing.T) {
	var tests = []struct {
		name        string
		path        string
		errExpected bool
	}{
		{name: "Create directory in existing project", path: "/tmp/mnt/mcfs/1/1/dir1", errExpected: false},
		{name: "Create directory off of dir1 should pass", path: "/tmp/mnt/mcfs/1/1/dir1/dir11", errExpected: false},
		{name: "Create directory in project that does not exist", path: "/tmp/mnt/mcfs/1/2/dir1", errExpected: true},
		{name: "Create directory where parent does not exist should fail", path: "/tmp/mnt/mcfs/1/1/dir2/dir3", errExpected: true},
	}

	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := os.Mkdir(test.path, 0755)
			if test.errExpected {
				require.Errorf(t, err, "Expected mkdir to fail for path %s", test.path)
			} else {
				require.NoErrorf(t, err, "Expected mkdir to succeed, got error %s for path %s", err, test.path)
				parent := filepath.Dir(test.path)
				createdDir := filepath.Base(test.path)
				entries, err := os.ReadDir(parent)
				require.NoErrorf(t, err, "Expected to read parent dir, got error %s for path %s", err, parent)
				require.Len(t, entries, 1)
				entry := entries[0]
				require.True(t, entry.IsDir())
				require.Equal(t, createdDir, entry.Name())
			}
		})
	}
}
