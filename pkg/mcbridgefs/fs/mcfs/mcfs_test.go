package mcfs

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListingProjects(t *testing.T) {
	tc := newTestCase(t, &fsTestOptions{mcfsDir: "/tmp/mcfs", mntDir: "/tmp/mnt/mcfs"})
	require.NotNil(t, tc)

	entries, err := os.ReadDir("/tmp/mnt/mcfs")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	fmt.Println("entries len = ", len(entries))
	entry := entries[0]
	require.True(t, entry.IsDir())
	require.Equal(t, "1", entry.Name())
}
