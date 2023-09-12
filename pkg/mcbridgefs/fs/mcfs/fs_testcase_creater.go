package mcfs

import (
	"fmt"
	"os"
	"testing"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fsTestCase struct {
	*testing.T
	mcfsDir string
	mntDir  string
	db      *gorm.DB

	mcfs   fs.InodeEmbedder
	rawFS  fuse.RawFileSystem
	server *fuse.Server
}

type fsTestOptions struct {
	entryCache    bool
	enableLocks   bool
	attrCache     bool
	suppressDebug bool
	dsn           string
	mcfsDir       string
	mntDir        string
}

func newTestCase(t *testing.T, opts *fsTestOptions) *fsTestCase {
	dsn := "file::memory:?cache=shared"
	if opts.dsn != "" {
		dsn = opts.dsn
	}
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})

	require.NotEmptyf(t, opts.mcfsDir, "opts.mcfsDir is not set")
	require.NotEmpty(t, opts.mntDir, "opts.mntDir is not set")

	require.NoError(t, err)
	require.NotNil(t, db)
	err = mcdb.RunMigrations(db)
	require.NoErrorf(t, err, "Migration failed with: %s", err)

	tc := &fsTestCase{
		T:       t,
		db:      db,
		mcfsDir: opts.mcfsDir,
		mntDir:  opts.mntDir,
	}

	if err := os.MkdirAll(opts.mcfsDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(opts.mntDir, 0755); err != nil {
		t.Fatal(err)
	}

	tc.mcfs, err = CreateFS(opts.mcfsDir, nil, nil)
	tc.rawFS = fs.NewNodeFS(tc.mcfs, &fs.Options{})
	tc.server, err = fuse.NewServer(tc.rawFS, opts.mntDir, &fuse.MountOptions{})
	if err != nil {
		t.Fatal(err)
	}

	go tc.server.Serve()
	if err := tc.server.WaitMount(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tc.unmount)
	return tc
}

func (tc *fsTestCase) unmount() {
	fmt.Println("unmount called now no sleep")
	if err := tc.server.Unmount(); err != nil {
		fmt.Println("tc.server.Unmount failed:", err)
		tc.Fatal(err)
	}

	//time.Sleep(time.Second *1)
	//fmt.Println("Calling umount")
	//cmd := exec.Command("/usr/bin/umount", tc.mntDir)
	//fmt.Println("Running command")
	//if err := cmd.Run(); err != nil {
	//	fmt.Println("unmount failed:", err)
	//}
	//fmt.Println("Run succeeded")
}
