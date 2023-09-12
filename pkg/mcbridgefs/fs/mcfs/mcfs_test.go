package mcfs

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSomething(t *testing.T) {
	tc := newTestCase(t, &fsTestOptions{mcfsDir: "/tmp/mcfs", mntDir: "/tmp/mnt/mcfs"})
	require.NotNil(t, tc)
	fmt.Println("Sleeping 3")
	time.Sleep(time.Second * 3)
	fmt.Println("... Done sleeping")
}
