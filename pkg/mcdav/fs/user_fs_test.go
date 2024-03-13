package fs

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanStatOpenAndListRoot(t *testing.T) {
	tc := newTestCase(t)

	finfo, err := tc.userFS.Stat(tc.ctx, "/")
	require.NoErrorf(t, err, "Failed calling userFS.Stat on /: %s", err)
	require.Truef(t, finfo.IsDir(), "FileInfo returned on Stat for / should be a directory, but isn't")
	require.Equalf(t, "/", finfo.Name(), "FileInfo returned on Stat for / should have name /, but got %s", finfo.Name())

	f, err := tc.userFS.OpenFile(tc.ctx, "/", os.O_RDONLY, 0755)
	require.NoErrorf(t, err, "Failed calling userFS.OpenFile on /: %s", err)

	finfos, err := f.Readdir(0)
	require.NoErrorf(t, err, "f.Readdir(0) returned error: %s", err)

	require.Len(t, finfos, 1, "Expected finfos for / to have len 1, instead there are %d finfo entries", len(finfos))

	finfo = finfos[0]

	require.Equalf(t, "proj1", finfo.Name(), "Expected name to be proj1 got %s", finfo.Name())
	require.Truef(t, finfo.IsDir(), "Expected finfo to be a directory")
}

func TestCanOpenProjectDirAndListIt(t *testing.T) {
	tc := newTestCase(t)

	// TestCase created a project proj1, so lets start from there
	finfo, err := tc.userFS.Stat(tc.ctx, "/proj1/")
	require.NoErrorf(t, err, "Failed calling userFS.Stat on /proj1/: %s", err)
	require.Truef(t, finfo.IsDir(), "FileInfo returned on Stat for /proj1/ should be a directory, but isn't")
	require.Equalf(t, "proj1", finfo.Name(), "FileInfo returned on Stat for /proj1 should have name proj1, but got %s", finfo.Name())

	f, err := tc.userFS.OpenFile(tc.ctx, "/proj1/", os.O_RDONLY, 0755)
	require.NoErrorf(t, err, "Failed calling userFS.OpenFile on /proj1/: %s", err)

	finfos, err := f.Readdir(0)
	require.NoErrorf(t, err, "f.Readdir(0) returned error: %s", err)

	require.Len(t, finfos, 1, "Expected finfos for / to have len 1, instead there are %d finfo entries", len(finfos))

	finfo = finfos[0]

	require.Equalf(t, "dir1", finfo.Name(), "Expecte name to be dir1 got %s", finfo.Name())
	require.Truef(t, finfo.IsDir(), "Expected finfo to be a directory")
}

func TestCanOpenSubDirNotProjectDirAndListIt(t *testing.T) {
	tc := newTestCase(t)

	// TestCase created a project proj1/dir1, so lets start from there
	finfo, err := tc.userFS.Stat(tc.ctx, "/proj1/dir1/")
	require.NoErrorf(t, err, "Failed calling userFS.Stat on /proj1/dir1/: %s", err)
	require.Truef(t, finfo.IsDir(), "FileInfo returned on Stat for /proj1/dir1/ should be a directory, but isn't")
	require.Equalf(t, "dir1", finfo.Name(), "FileInfo returned on Stat for /proj1/dir1/ should have name dir1, but got %s", finfo.Name())

	f, err := tc.userFS.OpenFile(tc.ctx, "/proj1/dir1/", os.O_RDONLY, 0755)
	require.NoErrorf(t, err, "Failed calling userFS.OpenFile on /proj1/dir1/: %s", err)

	finfos, err := f.Readdir(0)
	require.NoErrorf(t, err, "f.Readdir(0) returned error: %s", err)

	require.Len(t, finfos, 1, "Expected finfos for / to have len 1, instead there are %d finfo entries", len(finfos))

	finfo = finfos[0]

	require.Equalf(t, "test.txt", finfo.Name(), "Expecte name to be test.txt got %s", finfo.Name())
	require.Falsef(t, finfo.IsDir(), "Expected finfo not to be a directory")
}

func TestCreatingAFileInSubdir(t *testing.T) {
	tc := newTestCase(t)

	f, err := tc.userFS.OpenFile(tc.ctx, "/proj1/dir1/hello.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	require.NoErrorf(t, err, "userFS.OpenFile(%s) failed: %s", "/proj1/dir1/hello.txt", err)

	n, err := f.Write([]byte("hello"))
	require.NoErrorf(t, err, "f.Write failed: %s", err)
	require.Equalf(t, 5, n, "f.Write didn't write 5, it wrote %d", n)
	err = f.Close()
	require.NoErrorf(t, err, "f.Close failed: %s", err)

	// Make sure the file is in the database and looks correct
	file, err := tc.userFS.fileStor.GetFileByPath(tc.proj.ID, "/dir1/hello.txt")
	fmt.Printf("file = %#v\n", file)
	require.NoErrorf(t, err, "GetFileByPath(%s) failed: %s", "/dir1/hello.txt", err)
	require.True(t, file.Current, "file.Current isn't true")
	require.Equal(t, uint64(5), file.Size, "file.Size is not 5")
}

func TestCreatingAFileInProjectRoot(t *testing.T) {
	tc := newTestCase(t)

	f, err := tc.userFS.OpenFile(tc.ctx, "/proj1/hello.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	require.NoErrorf(t, err, "userFS.OpenFile(%s) failed: %s", "/proj1/hello.txt", err)

	n, err := f.Write([]byte("hello"))
	require.NoErrorf(t, err, "f.Write failed: %s", err)
	require.Equalf(t, 5, n, "f.Write didn't write 5, it wrote %d", n)
	err = f.Close()
	require.NoErrorf(t, err, "f.Close failed: %s", err)

	// Make sure the file is in the database and looks correct
	file, err := tc.userFS.fileStor.GetFileByPath(tc.proj.ID, "/hello.txt")
	fmt.Printf("file = %#v\n", file)
	require.NoErrorf(t, err, "GetFileByPath(%s) failed: %s", "/hello.txt", err)
	require.True(t, file.Current, "file.Current isn't true")
	require.Equal(t, uint64(5), file.Size, "file.Size is not 5")
}
