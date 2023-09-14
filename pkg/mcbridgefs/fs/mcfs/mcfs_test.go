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
	fmt.Println("entries len = ", len(entries))
}
