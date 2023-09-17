package mcfs

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListingTransferRequestProjects(t *testing.T) {
	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	entries, err := os.ReadDir("/tmp/mnt/mcfs")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	entry := entries[0]
	require.True(t, entry.IsDir())
	require.Equal(t, "1", entry.Name())

	// Test that project doesn't exist
	entries, err = os.ReadDir("/tmp/mnt/mcfs/2")
	require.Error(t, err)
}

func TestListingTransferRequestProjectUsers(t *testing.T) {
	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	// newTestCase creates a single project that has id 1, with a single user
	// with id 1
	entries, err := os.ReadDir("/tmp/mnt/mcfs/1")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	entry := entries[0]
	require.True(t, entry.IsDir())
	require.Equal(t, "1", entry.Name())
}
