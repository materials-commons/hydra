package mcfs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/fsstate"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/mcpath"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// fsTestCase represents a file system test case. It takes care of creating and mounting the file system
// populating the database, and unmounting the file system when the test is done.
type fsTestCase struct {
	*testing.T

	// mcfsDir is the directory where the files are stored
	mcfsDir string

	// mntDir is the directory that MCFS is mounted to
	mntDir string

	// The database connection
	db *gorm.DB

	// The stors that fsTestCase instantiates with db
	stors *stor.Stors

	// fuse/fs
	mcfs   fs.InodeEmbedder
	rawFS  fuse.RawFileSystem
	server *fuse.Server

	// services and db objects

	// The project created by the test case
	proj *mcmodel.Project

	// The user created by the test case
	user *mcmodel.User

	// The globus transfer created by the test case
	globusTransfer *mcmodel.GlobusTransfer

	// The transfer request created by the test case
	transferRequest *mcmodel.TransferRequest

	// The transferStateTracker used in the test case
	transferStateTracker *fsstate.TransferStateTracker

	// The factory for creating new MCFileHandles
	factory *MCFileHandlerFactory
}

type newPathParserFn func(stor *stor.Stors) mcpath.ParserReleaser

type fsTestOptions struct {
	entryCache    bool
	enableLocks   bool
	attrCache     bool
	suppressDebug bool

	// The dsn for the database. If blank it uses mcdb.SqliteInMemoryDSN
	dsn string

	// The directory to store files in. If blank it is set to /tmp/mcfs
	mcfsDir string

	// The directory to mount the file system to. If blank it is set to /tmp/mnt/mcfs
	mntDir string

	// path parser creater
	newPathParser newPathParserFn
}

type NullLogger struct{}

func (l *NullLogger) Printf(_ string, _ ...interface{}) {
}

// newTestStor creates a new Stor and DB does not create a file system. Populates the database.
func newTestStor(t *testing.T, dsnToUse, mcfsDir string) (*gorm.DB, *stor.Stors) {
	dsn := mcdb.SqliteInMemoryDSN
	if dsnToUse != "" {
		dsn = dsnToUse
		_ = os.Remove(dsnToUse)
		fh, err := os.Create(dsnToUse)
		require.NoErrorf(t, err, "Failed opening %s, got %s", dsnToUse, err)
		_ = fh.Close()
	}

	gormLogger := logger.New(&NullLogger{},
		logger.Config{
			SlowThreshold:             time.Second * 5,
			LogLevel:                  logger.Silent,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		})
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormLogger})
	require.NoErrorf(t, err, "gorm.Open failed: %s", err)
	sqlitedb, err := db.DB()
	sqlitedb.SetMaxOpenConns(1)
	err = mcdb.RunMigrations(db)
	require.NoErrorf(t, err, "Migration failed with: %s", err)

	tc := &fsTestCase{
		T:       t,
		db:      db,
		mcfsDir: mcfsDir,
		mntDir:  "",
	}

	tc.populateDatabase()

	return db, tc.stors
}

// newTestCase creates a new file system test. It mounts the file system, populates the
// database and sets up handlers for unmount when the test case ends. newTestCase also
// does state clean up at the start of test, by clearing out the mcfsDir, creating needed
// directories, attempting to unmount the mntDir in case the file system didn't unmount, etc...
func newTestCase(t *testing.T, opts *fsTestOptions) *fsTestCase {
	dsn := "file::memory:?cache=shared"
	if opts.dsn != "" {
		dsn = opts.dsn
	}

	if dsn != "file::memory:?cache=shared" {
		fmt.Printf("opening '%s'\n", dsn)
		_ = os.Remove(dsn)
		fh, err := os.Create(dsn)
		require.NoErrorf(t, err, "Failed opening %s, got %s", dsn, err)
		_ = fh.Close()
	}

	gormLogger := logger.New(&NullLogger{},
		logger.Config{
			SlowThreshold:             time.Second * 5,
			LogLevel:                  logger.Silent,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		})
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormLogger})
	require.NoErrorf(t, err, "gorm.Open failed: %s", err)
	sqlitedb, err := db.DB()

	// Set the sqlite db to 1 connection. This gets around table lock issues from
	// multiple threads.
	sqlitedb.SetMaxOpenConns(1)

	if opts.mcfsDir == "" {
		opts.mcfsDir = "/tmp/mcfs"
	}

	if opts.mntDir == "" {
		opts.mntDir = "/tmp/mnt/mcfs"
	}

	umount(opts.mntDir)

	_ = os.RemoveAll(opts.mcfsDir)

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

	tc.transferStateTracker = fsstate.NewTransferStateTracker()
	stors := stor.NewGormStors(tc.db, tc.mcfsDir)
	var pathParser mcpath.Parser
	if opts.newPathParser != nil {
		pathParser = opts.newPathParser(stors)
	} else {
		pathParser = mcpath.NewTransferPathParser(stors)
	}
	mcapi := NewLocalMCFSApi(stors, tc.transferStateTracker, pathParser, opts.mcfsDir)
	activityCounterMonitor := fsstate.NewActivityCounterMonitor(time.Second * 2)
	newHandleFactory := NewMCFileHandlerFactory(mcapi, tc.transferStateTracker, pathParser, activityCounterMonitor)
	tc.factory = newHandleFactory

	newFileHandleFunc := func(fd, flags int, path string, file *mcmodel.File) fs.FileHandle {
		return newHandleFactory.NewFileHandle(fd, flags, path, file)
	}

	tc.mcfs, err = CreateFS(opts.mcfsDir, mcapi, newFileHandleFunc)
	tc.rawFS = fs.NewNodeFS(tc.mcfs, &fs.Options{})
	tc.server, err = fuse.NewServer(tc.rawFS, opts.mntDir, &fuse.MountOptions{Name: "mcfs"})
	if err != nil {
		t.Fatal(err)
	}

	go tc.server.Serve()
	if err := tc.server.WaitMount(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tc.unmount)
	t.Cleanup(tc.closeDB)
	return tc
}

// populateDatabase is called from newTestCase and newTestStor. It populates the database
// with a project, user, globus transfer and transfer request. It saves these created items
// into the fsTestCase.
func (tc *fsTestCase) populateDatabase() {
	var err error

	tc.stors = stor.NewGormStors(tc.db, tc.mcfsDir)

	user := &mcmodel.User{Email: "user1@test.com"}

	tc.user, err = tc.stors.UserStor.CreateUser(user)
	require.NoErrorf(tc.T, err, "Failed creating user1: %s", err)

	proj := &mcmodel.Project{Name: "Proj1", OwnerID: user.ID}

	tc.proj, err = tc.stors.ProjectStor.CreateProject(proj)
	require.NoErrorf(tc.T, err, "Failed creating proj: %s", err)

	transferRequest := &mcmodel.TransferRequest{
		ProjectID: tc.proj.ID,
		OwnerID:   tc.proj.OwnerID,
		State:     "open",
	}

	tc.transferRequest, err = tc.stors.TransferRequestStor.CreateTransferRequest(transferRequest)
	require.NoError(tc.T, err)

	globusTransfer := &mcmodel.GlobusTransfer{
		ProjectID:         tc.proj.ID,
		State:             "open",
		OwnerID:           tc.proj.OwnerID,
		TransferRequestID: transferRequest.ID,
	}

	tc.globusTransfer, err = tc.stors.GlobusTransferStor.CreateGlobusTransfer(globusTransfer)
	require.NoError(tc.T, err)
}

// unmount is set as a test Cleanup callback to unmount the file system at the
// end of a test.
func (tc *fsTestCase) unmount() {
	if err := tc.server.Unmount(); err != nil {
		tc.Fatal(err)
	}
}

// closeDB is run after a test completes to close the underlying database. This
// ensures that the database is closed and for in memory sqlite databases that
// it can't be reused. This is important, because the in memory database ends
// up getting reused between tests, which creates multiple instances of the
// projects, users, etc... that populateDatabase is adding. Tests assume that
// there is only a single entry of these items and thus the test will fail.
func (tc *fsTestCase) closeDB() {
	time.Sleep(time.Millisecond * 5)
	sqlDB, err := tc.db.DB()

	if err != nil {
		return
	}

	if sqlDB == nil {
		return
	}

	_ = sqlDB.Close()
}

// umount is called at the beginning of a test to unmount previous runs. The two
// different unmount methods are used to handle consistent failure of one of the
// ways to unmount.
func umount(path string) {
	cmd := exec.Command("/usr/bin/umount", path)
	_ = cmd.Run()
}

func (tc *fsTestCase) makeTransferRequestPath(path string) string {
	return filepath.Join(tc.mntDir, tc.transferRequest.UUID, path)
}
