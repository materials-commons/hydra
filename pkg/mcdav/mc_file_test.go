package mcdav

import (
	"os"
	"testing"

	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCanStatRoot(t *testing.T) {
	tc := newMCFileTestCase(t)
	_ = tc
}

func TestCanStatAProject(t *testing.T) {
	t.Fatalf("not implemented")
}

func TestCanStatAFile(t *testing.T) {
	t.Fatalf("not implemented")
}

func TestCanListProjectsInRoot(t *testing.T) {
	t.Fatalf("not implemented")
}

type mcfileTestCase struct {
	*testing.T
	stors   *stor.Stors
	db      *gorm.DB
	mcfsDir string
	user    *mcmodel.User
	proj    *mcmodel.Project
	rootDir *mcmodel.File
	dir1    *mcmodel.File
	f       *mcmodel.File
}

func newMCFileTestCase(t *testing.T) *mcfileTestCase {
	dsn := mcdb.SqliteInMemoryDSN
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoErrorf(t, err, "gorm.Open failed: %s", err)
	sqlitedb, err := db.DB()
	sqlitedb.SetMaxOpenConns(1)
	err = mcdb.RunMigrations(db)
	tc := &mcfileTestCase{
		db:      db,
		T:       t,
		mcfsDir: "/tmp/mcfs",
	}

	_ = os.RemoveAll(tc.mcfsDir)

	_ = os.MkdirAll(tc.mcfsDir, 0755)

	require.NoErrorf(t, err, "Migration failed with: %s", err)

	tc.populateDatabase()

	return tc
}

func (tc *mcfileTestCase) populateDatabase() {
	var err error

	tc.stors = stor.NewGormStors(tc.db, tc.mcfsDir)

	user := &mcmodel.User{Email: "user1@test.com"}

	tc.user, err = tc.stors.UserStor.CreateUser(user)
	require.NoErrorf(tc.T, err, "Failed creating user1: %s", err)

	proj := &mcmodel.Project{Name: "Proj1", OwnerID: user.ID}

	proj, err = tc.stors.ProjectStor.CreateProject(proj)
	require.NoErrorf(tc.T, err, "Failed creating proj: %s", err)

	tc.rootDir, err = tc.stors.FileStor.GetDirByPath(proj.ID, "/")
	require.NoErrorf(tc.T, err, "Failed retrieving root dir for project %d: %s", proj.ID, err)

	tc.dir1, err = tc.stors.FileStor.CreateDirectory(tc.rootDir.ID, proj.ID, tc.user.ID, "/dir1", "dir1")
	require.NoErrorf(tc.T, err, "Failed creating dir off of root: %s", err)

	tc.f, err = tc.stors.FileStor.CreateFile("test.txt", proj.ID, tc.dir1.ID, tc.user.ID, "text/text")
	require.NoErrorf(tc.T, err, "Failed creating file test.txt in dir %s: %s", tc.dir1.Path, err)

	err = tc.f.MkdirUnderlyingPath(tc.mcfsDir)
	require.NoErrorf(tc.T, err, "Failed creating underlying directory for file %s: %s", tc.f.ToUnderlyingDirPath(tc.mcfsDir), err)

	err = os.WriteFile(tc.f.ToUnderlyingFilePath(tc.mcfsDir), []byte("Test data"), 0777)
	require.NoErrorf(tc.T, err, "Unable to write data for file %s: %s", tc.f.ToUnderlyingFilePath(tc.mcfsDir), err)
}
