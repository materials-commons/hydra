package mcfs

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListingTransferRequestProjects(t *testing.T) {
	var tests = []struct {
		name        string
		dir         string
		numEntries  int
		errExpected bool
	}{
		{name: "list projects", dir: "/tmp/mnt/mcfs", numEntries: 1, errExpected: false},
		{name: "project does not exist", dir: "/tmp/mnt/mcfs/2", numEntries: 0, errExpected: true},
	}

	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries, err := os.ReadDir(test.dir)
			if test.errExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, entries, test.numEntries)
				// test only returns one project, with id 1
				entry := entries[0]
				require.True(t, entry.IsDir())
				require.Equal(t, "1", entry.Name())
			}
		})
	}
}

func TestListingTransferRequestProjectUsers(t *testing.T) {
	// newTestCase creates a single project that has id 1, with a single user
	// with id 1

	var tests = []struct {
		name        string
		dir         string
		numEntries  int
		errExpected bool
	}{
		{name: "test existing user exists", dir: "/tmp/mnt/mcfs/1", numEntries: 1, errExpected: false},
		{name: "test user does not exist", dir: "/tmp/mnt/mcfs/2", numEntries: 0, errExpected: true},
	}

	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries, err := os.ReadDir(test.dir)
			if test.errExpected {
				require.Errorf(t, err, "Should have gotten an error when reading %s", test.dir)
			} else {
				require.NoError(t, err)
				require.Len(t, entries, 1)
				entry := entries[0]
				require.True(t, entry.IsDir())
				require.Equal(t, "1", entry.Name())
			}
		})
	}
}

func TestLookup(t *testing.T) {
	// newTestCase will create a single project id 1, with a single user with id 1.
	// Lookup will look for all parent paths. We are going to check that look up is
	// working by doing a stat on these items.
	var tests = []struct {
		name        string
		path        string
		errExpected bool
	}{
		{name: "Check that project 1 exists", path: "/tmp/mnt/mcfs/1", errExpected: false},
		{name: "Check that project 2 does not exist", path: "/tmp/mnt/mcfs/2", errExpected: true},
		{name: "Check that project 1/user 1 exists", path: "/tmp/mnt/mcfs/1/1", errExpected: false},
		{name: "Check that project 1/user 2 does not exist", path: "/tmp/mnt/mcfs/1/2", errExpected: true},
	}

	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			finfo, err := os.Stat(test.path)
			if test.errExpected {
				require.Errorf(t, err, "Expected err for path %s", test.path)
			} else {
				require.NoErrorf(t, err, "Expected no error for path %s, got %s", test.path, err)
				require.Truef(t, finfo.IsDir(), "Expected %s to be a dir", test.path)
			}
		})
	}
}

func TestMkdir(t *testing.T) {
	var tests = []struct {
		name        string
		path        string
		errExpected bool
	}{
		{name: "Create directory in existing project", path: "/tmp/mnt/mcfs/1/1/dir1", errExpected: false},
		{name: "Create directory off of dir1 should pass", path: "/tmp/mnt/mcfs/1/1/dir1/dir11", errExpected: false},
		{name: "Create directory in project that does not exist", path: "/tmp/mnt/mcfs/1/2/dir1", errExpected: true},
		{name: "Create directory where parent does not exist should fail", path: "/tmp/mnt/mcfs/1/1/dir2/dir3", errExpected: true},
	}

	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := os.Mkdir(test.path, 0755)
			if test.errExpected {
				require.Errorf(t, err, "Expected mkdir to fail for path %s", test.path)
			} else {
				require.NoErrorf(t, err, "Expected mkdir to succeed, got error %s for path %s", err, test.path)
				parent := filepath.Dir(test.path)
				createdDir := filepath.Base(test.path)
				entries, err := os.ReadDir(parent)
				require.NoErrorf(t, err, "Expected to read parent dir, got error %s for path %s", err, parent)
				require.Len(t, entries, 1)
				entry := entries[0]
				require.True(t, entry.IsDir())
				require.Equal(t, createdDir, entry.Name())
			}
		})
	}
}

func TestCreate(t *testing.T) {
	var tests = []struct {
		name        string
		path        string
		projectID   int
		projectPath string
		errExpected bool
	}{
		{
			name:        "Can create file in existing transfer",
			path:        "/tmp/mnt/mcfs/1/1/create.txt",
			projectPath: "/create.txt",
			projectID:   1,
			errExpected: false,
		},
		{
			name:        "Should creating a file when transfer path is invalid",
			path:        "/tmp/mnt/mcfs/1/2/fail.txt",
			projectPath: "/fail.txt",
			projectID:   1,
			errExpected: true,
		},
	}

	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fh, err := os.Create(test.path)

			if test.errExpected {
				require.Errorf(t, err, "Expected error for path %s", test.path)
			} else {
				require.NoErrorf(t, err, "Expected no error, got %s for path %s", err, test.path)

				numBytes, err := io.WriteString(fh, test.path)
				fh.Close()

				require.Equalf(t, numBytes, len(test.path), "Wrong length expected %d, got %d", numBytes, len(test.path))
				// Assume all paths are written to the root
				f, err := tc.stors.FileStor.GetFileByPath(test.projectID, test.projectPath)
				require.NoErrorf(t, err, "Failed getting file")
				require.True(t, f.Current)
				hasher := md5.New()
				_, _ = io.Copy(hasher, strings.NewReader(test.path))
				checksum := fmt.Sprintf("%x", hasher.Sum(nil))
				require.Equal(t, checksum, f.Checksum)
				require.Equal(t, len(test.path), int(f.Size))
				require.NotEmpty(t, f.MimeType)
			}
		})
	}
}

func TestOpen(t *testing.T) {
	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	// Write then read to make sure we get the same results
	path := "/tmp/mnt/mcfs/1/1/readwrite.txt"

	fh, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	require.NoErrorf(t, err, "Unexpected error on open %s, for path %s", err, path)
	what := "readwrite"
	n, err := io.WriteString(fh, what)
	require.NoErrorf(t, err, "Got unexpected error on write: %s", err)
	require.Equal(t, len(what), n)
	fh.Close()
	f, err := tc.stors.FileStor.GetFileByPath(1, "/readwrite.txt")
	require.NoErrorf(t, err, "Couldn't get database file entry: %s", err)

	// Reopen and make sure we get back what was written
	fmt.Println("Calling os.OpenFile")
	fh, err = os.OpenFile(path, os.O_RDONLY, 0755)
	fmt.Println("Past os.OpenFile")
	require.NoErrorf(t, err, "Unexpected error on open %s, for path %s", err, path)
	contents := make([]byte, 1024)
	n, err = fh.Read(contents)
	require.NoErrorf(t, err, "Got error on read %s", err)
	require.Equal(t, len(what), n)
	require.Equal(t, what, string(contents[:n]))
	fh.Close()
	f2, err := tc.stors.FileStor.GetFileByPath(1, "/readwrite.txt")
	require.NoErrorf(t, err, "Couldn't get database file entry: %s", err)

	// At this point the database knownFiles should be the same
	require.Equal(t, f.ID, f2.ID)
}

func TestFileTruncation(t *testing.T) {
	// When a file is opened for truncation what is the flow?
	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	path := "/tmp/mnt/mcfs/1/1/trunctest.txt"

	// Test Open with O_TRUNC
	fh, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	require.NoErrorf(t, err, "Got error opening for truncate: %s", err)

	what := "will truncate content"
	n, err := io.WriteString(fh, what)
	require.NoErrorf(t, err, "Got error on io.WriteString: %s", err)
	require.Equal(t, len(what), n)
	err = fh.Close()
	require.NoError(t, err)

	fh2, err := os.OpenFile(path, os.O_RDWR|os.O_TRUNC, 0755)
	require.NoErrorf(t, err, "Got error opening for truncate: %s", err)
	what = "Truncated!"
	n, err = io.WriteString(fh2, what)
	require.NoErrorf(t, err, "Got error on io.WriteString: %s", err)
	require.Equal(t, len(what), n)
	err = fh2.Close()
	require.NoError(t, err)

	// Test Using FTruncate
	fh, err = os.OpenFile(path, os.O_RDWR, 0755)
	require.NoError(t, err)
	fd := fh.Fd()
	err = syscall.Ftruncate(int(fd), 0)
	require.NoError(t, err)
	st := syscall.Stat_t{}
	err = syscall.Fstat(int(fd), &st)
	require.NoError(t, err)
	require.Equal(t, int64(0), st.Size)
	err = fh.Close()
	require.NoError(t, err)
}

func TestStatfs(t *testing.T) {
	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	st := syscall.Statfs_t{}
	err := syscall.Statfs("/tmp/mnt/mcfs", &st)
	require.NoError(t, err)
}

func TestStat(t *testing.T) {
	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	path := "/tmp/mnt/mcfs/1/1/file.txt"
	fh, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	require.NoErrorf(t, err, "Got error opening for truncate: %s", err)

	what := "will truncate content"
	n, err := io.WriteString(fh, what)
	require.NoErrorf(t, err, "Got error on io.WriteString: %s", err)
	require.Equal(t, len(what), n)
	err = fh.Close()
	require.NoError(t, err)

	finfo, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, int64(n), finfo.Size())
}

func TestParallelReadWrite(t *testing.T) {
	var lines [3]string = [3]string{"abc", "def", "ghi"}
	expected := "abcdefghi"

	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	var wg sync.WaitGroup

	fn := func(testNumber int) {
		defer wg.Done()
		path := fmt.Sprintf("/tmp/mnt/mcfs/1/1/file%d.txt", testNumber)
		fh, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
		assert.NoError(t, err)
		for _, line := range lines {
			n, err := io.WriteString(fh, line)
			assert.NoError(t, err)
			assert.Equal(t, n, len(line))
		}
		n, err := io.WriteString(fh, path)
		assert.NoError(t, err)
		assert.Equal(t, n, len(path))
		err = fh.Close()
		assert.NoError(t, err)
		fh, err = os.OpenFile(path, os.O_RDONLY, 0755)
		assert.NoError(t, err)
		contents, err := os.ReadFile(path)
		assert.NoError(t, err)
		texpected := expected + path
		assert.Equal(t, texpected, string(contents))
		assert.NoError(t, fh.Close())
	}

	for i := 0; i < 100; i++ {
		testNumber := i
		wg.Add(1)
		go fn(testNumber)
	}
	wg.Wait()
}

func TestParallelMkdirSameUser(t *testing.T) {
	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	dirToCreate := "/tmp/mnt/mcfs/1/1/dir1"
	var wg sync.WaitGroup
	fn := func() {
		defer wg.Done()
		if err := os.Mkdir(dirToCreate, 0755); err != nil {
			assert.True(t, errors.Is(err, os.ErrExist))
		}
	}
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go fn()
	}
	wg.Wait()

	var count int64
	err := tc.db.Model(&mcmodel.File{}).Where("name", "dir1").Count(&count).Error
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func TestParallelMkdirDifferentUser(t *testing.T) {
	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	// Create a second transfer request and user, so we can do
	// multiple mkdirs across the users
	var err error
	user := &mcmodel.User{Email: "user2@test.com"}
	user, err = tc.stors.UserStor.CreateUser(user)
	require.NoError(t, err)

	err = tc.stors.ProjectStor.AddMemberToProject(tc.proj, user)
	require.NoError(t, err)

	transferRequest := &mcmodel.TransferRequest{
		ProjectID: tc.proj.ID,
		OwnerID:   user.ID,
		State:     "open",
	}

	transferRequest, err = tc.stors.TransferRequestStor.CreateTransferRequest(transferRequest)
	require.NoError(t, err)

	globusTransfer := &mcmodel.GlobusTransfer{
		ProjectID:         tc.proj.ID,
		State:             "open",
		OwnerID:           user.ID,
		TransferRequestID: transferRequest.ID,
	}

	_, err = tc.stors.GlobusTransferStor.CreateGlobusTransfer(globusTransfer)
	require.NoError(t, err)

	// Now run the test

	var wg sync.WaitGroup
	fnUser1 := func() {
		defer wg.Done()
		dirToCreate := "/tmp/mnt/mcfs/1/1/dir1"
		if err := os.Mkdir(dirToCreate, 0755); err != nil {
			assert.True(t, errors.Is(err, os.ErrExist))
		}
	}

	fnUser2 := func() {
		defer wg.Done()
		dirToCreate := "/tmp/mnt/mcfs/1/2/dir1"
		if err := os.Mkdir(dirToCreate, 0755); err != nil {
			assert.True(t, errors.Is(err, os.ErrExist))
		}
	}

	for i := 0; i < 1000; i++ {
		// Add to waitgroup twice, once for each func
		wg.Add(1)
		wg.Add(1)
		go fnUser1()
		go fnUser2()
	}
	wg.Wait()

	var count int64
	err = tc.db.Model(&mcmodel.File{}).Where("name", "dir1").Count(&count).Error
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func TestActivityCounterIsIncrementedOnReadsAndWrites(t *testing.T) {
	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	// Test that write will increment the activity counter
	path := "/tmp/mnt/mcfs/1/1/file.txt"
	fh, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	require.NoErrorf(t, err, "Got error opening for truncate: %s", err)

	_, err = io.WriteString(fh, "hello ")
	require.NoError(t, err)

	// Stored by project/user path, eg /1/1
	activityCounter, found := tc.factory.activityCounterFactory.activityCounters["/1/1"]
	require.True(t, found)

	count := atomic.LoadInt64(&(activityCounter.activityCount))
	require.Equal(t, int64(1), count)

	_, _ = io.WriteString(fh, "world")
	count = atomic.LoadInt64(&(activityCounter.activityCount))
	require.Equal(t, int64(2), count)
	require.NoError(t, fh.Close())

	_, err = os.ReadFile(path)
	require.NoErrorf(t, err, "Got error opening for truncate: %s", err)
	count = atomic.LoadInt64(&(activityCounter.activityCount))
	require.Equal(t, int64(3), count)
}

func TestMonitorHandlesInactivityDeadline(t *testing.T) {
	// Test the function to determine if inactivity time has passed
	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	// Test that write will increment the activity counter
	path := "/tmp/mnt/mcfs/1/1/file.txt"
	fh, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	require.NoErrorf(t, err, "Got error opening for truncate: %s", err)
	require.NoError(t, fh.Close())

	//time.Sleep(time.Second * 4)
	//// Check if expired by seeing if time is ahead of write by seconds
	//now := time.Now()
	//
	//activityCounter, found := tc.factory.activityCounterFactory.activityCounters["/1/1"]
	//require.True(t, found)
	//
	//require.True(t, now.After(activityCounter.LastChanged))

}
