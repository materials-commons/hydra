package mcfs

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fsTestCase struct {
	*testing.T

	mcfsDir string
	mntDir  string
	db      *gorm.DB
	stors   *stor.Stors

	// fuse/fs
	mcfs   fs.InodeEmbedder
	rawFS  fuse.RawFileSystem
	server *fuse.Server

	// database objects
	proj            *mcmodel.Project
	user            *mcmodel.User
	globusTransfer  *mcmodel.GlobusTransfer
	transferRequest *mcmodel.TransferRequest
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

	if opts.mcfsDir == "" {
		opts.mcfsDir = "/tmp/mcfs"
	}

	if opts.mntDir == "" {
		opts.mntDir = "/tmp/mnt/mcfs"
	}

	fmt.Printf("opts = %+v\n", opts)
	umount(opts.mntDir)

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

	tc.populateDatabase()

	if err := os.MkdirAll(opts.mcfsDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(opts.mntDir, 0755); err != nil {
		t.Fatal(err)
	}

	mcApi := NewMCApi(stor.NewGormStors(tc.db, tc.mcfsDir), NewKnownFilesTracker())
	tc.mcfs, err = CreateFS(opts.mcfsDir, mcApi, nil)
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

func (tc *fsTestCase) populateDatabase() {
	// create users
	// foreach user
	//   create projects
	// foreach project
	//   create files and directories

	var err error

	tc.stors = stor.NewGormStors(tc.db, tc.mcfsDir)

	user := &mcmodel.User{Email: "user1@test.com"}

	tc.user, err = tc.stors.UserStor.CreateUser(user)
	require.NoErrorf(tc.T, err, "Failed creating user1: %s", err)

	proj := &mcmodel.Project{Name: "Proj1", OwnerID: user.ID}

	tc.proj, err = tc.stors.ProjectStor.CreateProject(proj)
	require.NoErrorf(tc.T, err, "Failed creating proj1: %s", err)
	transferRequest := &mcmodel.TransferRequest{
		ProjectID: tc.proj.ID,
		OwnerID:   tc.proj.OwnerID,
		State:     "open",
	}

	tc.transferRequest, err = tc.stors.TransferRequestStor.CreateTransferRequest(transferRequest)
	require.NoError(tc.T, err)

	globusTransfer := &mcmodel.GlobusTransfer{
		ProjectID:         0,
		State:             "open",
		OwnerID:           tc.proj.OwnerID,
		TransferRequestID: transferRequest.ID,
	}

	tc.globusTransfer, err = tc.stors.GlobusTransferStor.CreateGlobusTransfer(globusTransfer)
	require.NoError(tc.T, err)
}

func (tc *fsTestCase) unmount() {
	if err := tc.server.Unmount(); err != nil {
		fmt.Println("tc.server.Unmount failed:", err)
		tc.Fatal(err)
	}
}

func umount(path string) {
	cmd := exec.Command("/usr/bin/umount", path)
	if err := cmd.Run(); err != nil {
		//fmt.Println("umount failed:", err)
	}
}
