package mcfs

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

	// services and db objects
	proj              *mcmodel.Project
	user              *mcmodel.User
	globusTransfer    *mcmodel.GlobusTransfer
	transferRequest   *mcmodel.TransferRequest
	knownFilesTracker *KnownFilesTracker
	factory           *MCFileHandlerFactory
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

type NullLogger struct{}

func (l *NullLogger) Printf(_ string, _ ...interface{}) {
	//fmt.Println("Null logger called")
	// do nothing
}

func newTestStor(t *testing.T, dsnToUse, mcfsDir string) (*gorm.DB, *stor.Stors) {
	dsn := mcdb.SqliteInMemoryDSN
	if dsnToUse != "" {
		dsn = dsnToUse
		_ = os.Remove(dsnToUse)
		fh, err := os.Create(dsnToUse)
		require.NoErrorf(t, err, "Failed opening %s, got %s", dsnToUse, err)
		fh.Close()
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

func newTestCase(t *testing.T, opts *fsTestOptions) *fsTestCase {
	dsn := "file::memory:?cache=shared"
	if opts.dsn != "" {
		dsn = opts.dsn
		_ = os.Remove(opts.dsn)
		fh, err := os.Create(opts.dsn)
		require.NoErrorf(t, err, "Failed opening %s, got %s", opts.dsn, err)
		fh.Close()
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

	tc.knownFilesTracker = NewKnownFilesTracker()
	stors := stor.NewGormStors(tc.db, tc.mcfsDir)
	mcapi := NewMCApi(stors, tc.knownFilesTracker)
	newHandleFactory := NewMCFileHandlerFactory(mcapi, tc.knownFilesTracker, time.Second*2)
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
	sqlDB, err := tc.db.DB()

	if err != nil {
		return
	}

	if sqlDB == nil {
		return
	}

	sqlDB.Close()
}

func umount(path string) {
	cmd := exec.Command("/usr/bin/umount", path)
	_ = cmd.Run()
}
