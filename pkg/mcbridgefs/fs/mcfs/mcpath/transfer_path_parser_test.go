package mcpath

import (
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestTransferPathParser_Parse(t *testing.T) {
	tc := newTransferPathParserTestCase(t, "")

	var tests = []struct {
		name                 string
		path                 string
		errExpected          bool
		pathTypeExpected     PathType
		fullPathExpected     string
		transferBaseExpected string
		projectPathExpected  string
	}{
		{
			name:                 "root path",
			path:                 "/",
			errExpected:          false,
			pathTypeExpected:     RootPath,
			fullPathExpected:     "/",
			transferBaseExpected: "",
			projectPathExpected:  "",
		},
		{
			name:                 "transfer request path",
			path:                 tc.transferRequest.Join(),
			errExpected:          false,
			pathTypeExpected:     ContextPath,
			fullPathExpected:     filepath.Join("/", tc.transferRequest.UUID),
			transferBaseExpected: filepath.Join("/", tc.transferRequest.UUID),
			projectPathExpected:  "",
		},
		{
			name:                 "project path",
			path:                 tc.transferRequest.Join("/dir1/file1.txt"),
			errExpected:          false,
			pathTypeExpected:     ProjectPath,
			fullPathExpected:     filepath.Join("/", tc.transferRequest.UUID, "dir1/file1.txt"),
			transferBaseExpected: filepath.Join("/", tc.transferRequest.UUID),
			projectPathExpected:  "/dir1/file1.txt",
		},
		{
			name:                 "project path",
			path:                 tc.transferRequest.Join("/dir1"),
			errExpected:          false,
			pathTypeExpected:     ProjectPath,
			fullPathExpected:     filepath.Join("/", tc.transferRequest.UUID, "dir1"),
			transferBaseExpected: filepath.Join("/", tc.transferRequest.UUID),
			projectPathExpected:  "/dir1",
		},
		{
			name:                 "unclean path",
			path:                 filepath.Join("/", tc.transferRequest.UUID, "/dir1/../dir1/file.txt"),
			errExpected:          false,
			pathTypeExpected:     ProjectPath,
			fullPathExpected:     filepath.Join("/", tc.transferRequest.UUID, "dir1", "file.txt"),
			transferBaseExpected: filepath.Join("/", tc.transferRequest.UUID),
			projectPathExpected:  "/dir1/file.txt",
		},
		{
			name:             "invalid uuid",
			path:             "/abc123/dir1",
			errExpected:      true,
			pathTypeExpected: BadPath,
		},
	}

	transferPathParser := NewTransferPathParser(tc.stors.TransferRequestStor)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tp, err := transferPathParser.Parse(test.path)
			if test.errExpected {
				require.Error(t, err)
				require.Equal(t, BadPath, tp.PathType())
				return
			}

			require.NoError(t, err)

			if test.pathTypeExpected == RootPath {
				require.Equal(t, test.pathTypeExpected, tp.PathType())
				require.Equal(t, test.fullPathExpected, tp.FullPath())
				require.Equal(t, test.transferBaseExpected, tp.TransferBase())
				require.Equal(t, "", tp.TransferUUID())
				require.Equal(t, -1, tp.TransferID())
				require.Equal(t, -1, tp.ProjectID())
				require.Equal(t, -1, tp.UserID())
				return
			}

			// If we are here then there is a transfer request associated with the path
			require.Equal(t, test.pathTypeExpected, tp.PathType())
			require.Equal(t, test.fullPathExpected, tp.FullPath())
			require.Equal(t, test.transferBaseExpected, tp.TransferBase())
			require.Equal(t, test.projectPathExpected, tp.ProjectPath())
			require.Equal(t, tc.transferRequest.UUID, tp.TransferUUID())
			require.Equal(t, tc.transferRequest.ID, tp.TransferID())
			require.Equal(t, tc.transferRequest.ProjectID, tp.ProjectID())
			require.Equal(t, tc.transferRequest.OwnerID, tp.UserID())
		})
	}
}

type transferPathParserTestCase struct {
	*testing.T
	db              *gorm.DB
	transferRequest *mcmodel.TransferRequest
	stors           *stor.Stors
	user            *mcmodel.User
	proj            *mcmodel.Project
	globusTransfer  *mcmodel.GlobusTransfer
}

func newTransferPathParserTestCase(t *testing.T, dsn string) *transferPathParserTestCase {
	if dsn == "" {
		dsn = mcdb.SqliteInMemoryDSN
	}

	gormLogger := logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags),
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

	tc := &transferPathParserTestCase{
		T:  t,
		db: db,
	}

	// Set the sqlite db to 1 connection. This gets around table lock issues from
	// multiple threads.
	sqlitedb.SetMaxOpenConns(1)

	err = mcdb.RunMigrations(db)
	require.NoErrorf(t, err, "Migration failed with: %s", err)

	tc.populateDatabase()
	t.Cleanup(func() {
		time.Sleep(time.Millisecond)
		sqlDB, err := tc.db.DB()

		if err != nil {
			return
		}

		if sqlDB == nil {
			return
		}

		_ = sqlDB.Close()
	})

	return tc
}

// populateDatabase is called from newTestCase and newTestStor. It populates the database
// with a project, user, globus transfer and transfer request. It saves these created items
// into the fsTestCase.
func (tc *transferPathParserTestCase) populateDatabase() {
	var err error

	tc.stors = stor.NewGormStors(tc.db, "")

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
