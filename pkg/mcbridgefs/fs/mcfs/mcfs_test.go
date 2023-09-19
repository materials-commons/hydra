package mcfs

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

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
				// Give the file system time to call and finish Release on the file
				time.Sleep(time.Second * 1)

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
	var tests = []struct {
		name        string
		path        string
		openFlags   int
		projectID   int
		projectPath string
		errExpected bool
	}{
		{
			name:        "test open read/write",
			path:        "/tmp/mnt/mcfs/1/1/readwrite.txt",
			openFlags:   os.O_RDWR | os.O_CREATE,
			projectID:   1,
			projectPath: "/readwrite.txt",
			errExpected: false,
		},
		{
			name:        "test open read - open for read readwrite.txt",
			path:        "/tmp/mnt/mcfs/1/1/readwrite.txt",
			openFlags:   os.O_RDONLY,
			projectID:   1,
			projectPath: "/readwrite.txt",
			errExpected: false,
		},
		//{
		//	name:        "test open write",
		//	path:        "/tmp/mnt/mcfs/1/1/write.txt",
		//	openFlags:   os.O_WRONLY | os.O_CREATE,
		//	projectID:   1,
		//	projectPath: "/readwrite.txt",
		//	errExpected: false,
		//},
		//{
		//	name:        "test re-open write - re-open for write write.txt",
		//	path:        "/tmp/mnt/mcfs/1/1/write.txt",
		//	openFlags:   os.O_WRONLY,
		//	projectID:   1,
		//	projectPath: "/readwrite.txt",
		//	errExpected: false,
		//},
	}

	tc := newTestCase(t, &fsTestOptions{})
	require.NotNil(t, tc)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.errExpected {

			} else {
				fh, err := os.OpenFile(test.path, test.openFlags, 0755)
				require.NoErrorf(t, err, "Unexpected error on open %s, for path %s", err, test.path)
				omode := test.openFlags & syscall.O_ACCMODE

				if omode == os.O_RDONLY {
					contents := make([]byte, 1024)
					n, err := fh.Read(contents)
					fh.Close()
					require.NoErrorf(t, err, "Got error on read %s", err)
					fmt.Printf("!!!On Read read %d\n", n)
					fmt.Printf("file contents = %q\n", string(contents[:n]))
				} else {
					n, err := io.WriteString(fh, test.path)
					require.NoErrorf(t, err, "Got error writing to file %s", err)
					fmt.Printf("!!!Wrote %d\n", n)
					fh.Close()
					// Give the file system time to call and finish Release on the file
					time.Sleep(time.Second * 2)
					fmt.Println("done sleeping")
				}
				//	numBytes, err := io.WriteString(fh, test.path)
				//	fh.Close()
				//	// Give the file system time to call and finish Release on the file
				//	time.Sleep(time.Second * 1)
				//
				//	require.Equalf(t, numBytes, len(test.path), "Wrong length expected %d, got %d", numBytes, len(test.path))
				//	// Assume all paths are written to the root
				//	f, err := tc.stors.FileStor.GetFileByPath(test.projectID, test.projectPath)
				//	require.NoErrorf(t, err, "Failed getting file")
				//	require.True(t, f.Current)
				//	hasher := md5.New()
				//	_, _ = io.Copy(hasher, strings.NewReader(test.path))
				//	checksum := fmt.Sprintf("%x", hasher.Sum(nil))
				//	require.Equal(t, checksum, f.Checksum)
				//	require.Equal(t, len(test.path), int(f.Size))
				//	require.NotEmpty(t, f.MimeType)
				//}
			}
		})
	}
}
