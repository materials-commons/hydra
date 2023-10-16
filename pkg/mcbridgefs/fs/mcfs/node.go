package mcfs

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/apex/log"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type NewFileHandleFN func(fd, flags int, path string, file *mcmodel.File) fs.FileHandle

type RootData struct {
	mcfsRoot      string
	mcfsapi       MCFSApi
	uid           uint32
	gid           uint32
	newFileHandle NewFileHandleFN
	mu            sync.Mutex
}

type Node struct {
	fs.Inode
	RootData *RootData
}

func CreateFS(fsRoot string, mcfsapi MCFSApi, fn NewFileHandleFN) (*Node, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	uid32, _ := strconv.ParseUint(u.Uid, 10, 32)
	gid32, _ := strconv.ParseUint(u.Gid, 10, 32)

	rootData := &RootData{
		mcfsRoot:      fsRoot,
		uid:           uint32(uid32),
		gid:           uint32(gid32),
		mcfsapi:       mcfsapi,
		newFileHandle: fn,
	}

	n := &Node{RootData: rootData}
	return n, nil
}

func (n *Node) newNode() *Node {
	return &Node{
		RootData: n.RootData,
	}
}

// Readdir reads the corresponding directory and returns its knownFiles
func (n *Node) Readdir(_ context.Context) (ds fs.DirStream, errno syscall.Errno) {
	defer func() {
		if r := recover(); r != nil {
			ds = nil
			errno = syscall.ENOENT
		}
	}()

	dirPath := filepath.Join("/", n.Path(n.Root()))
	files, err := n.RootData.mcfsapi.Readdir(dirPath)
	if err != nil {
		return nil, syscall.ENOENT
	}

	filesList := make([]fuse.DirEntry, 0, len(files))
	for _, f := range files {
		entry := fuse.DirEntry{
			Mode: n.getMode(&f),
			Name: f.Name,
			Ino:  n.inodeHash(dirPath, &f),
		}

		filesList = append(filesList, entry)
	}

	return fs.NewListDirStream(filesList), fs.OK
}

// Opendir just returns success
func (n *Node) Opendir(_ context.Context) syscall.Errno {
	return fs.OK
}

// Getxattr returns extra attributes. This is used by lstat. There are no extra attributes to
// return, so we always return a 0 for buffer length and success.
func (n *Node) Getxattr(_ context.Context, _ string, _ []byte) (uint32, syscall.Errno) {
	//log.Error("Node.Getxattr")
	return 0, fs.OK
}

// Getattr gets attributes about the file
func (n *Node) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) (errno syscall.Errno) {
	log.Debug("Node.Getattr")
	defer func() {
		if r := recover(); r != nil {
			errno = syscall.ENOENT
		}
	}()

	// Owner is always the process the bridge is running as
	out.Uid = n.RootData.uid
	out.Gid = n.RootData.gid

	if n.IsDir() {
		now := time.Now()
		out.SetTimes(&now, &now, &now)
		return fs.OK
	}

	if fops, ok := f.(fs.FileGetattrer); ok {
		return fops.Getattr(ctx, out)
	}

	// If we are here then f was nil, so we have to do lookups based on the path
	path := filepath.Join("/", n.Path(n.Root()))
	realPath, err := n.RootData.mcfsapi.GetRealPath(path)
	if err != nil {
		return syscall.ENOENT
	}

	st := syscall.Stat_t{}
	if err := syscall.Lstat(realPath, &st); err != nil {
		log.Errorf("Node.Getattr: Lstat failed path %s: %s", realPath, err)
		return fs.ToErrno(err)
	}

	out.FromStat(&st)

	return fs.OK
}

// Lookup will return information about the current entry.
func (n *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (inode *fs.Inode, errno syscall.Errno) {
	n.RootData.mu.Lock()
	defer func() {
		if r := recover(); r != nil {
			inode = nil
			errno = syscall.ENOENT
		}
		n.RootData.mu.Unlock()
	}()

	dirPath := filepath.Join("/", n.Path(n.Root()))
	path := filepath.Join(dirPath, name)

	if path == "/" {
		// Root dir, send back a default entry
		node := n.newNode()
		mode := 0755 | uint32(syscall.S_IFDIR)
		return n.NewInode(ctx, node, fs.StableAttr{Mode: mode}), fs.OK
	}

	f, err := n.RootData.mcfsapi.Lookup(path)
	if err != nil {
		fmt.Printf("Lookup failed %s: %s\n", path, err)
		return nil, syscall.ENOENT
	}

	out.Uid = n.RootData.uid
	out.Gid = n.RootData.gid
	if f.IsFile() {
		out.Size = f.Size
	}

	now := time.Now()
	out.SetTimes(&now, &f.UpdatedAt, &now)

	node := n.newNode()
	return n.NewInode(ctx, node, fs.StableAttr{Mode: n.getMode(f)}), fs.OK
}

// Mkdir will create a new directory. If an attempt is made to create an existing directory then it will return
// the existing directory rather than returning an error.
func (n *Node) Mkdir(ctx context.Context, name string, _ uint32, out *fuse.EntryOut) (inode *fs.Inode, errno syscall.Errno) {
	defer func() {
		if r := recover(); r != nil {
			inode = nil
			errno = syscall.EINVAL
		}
		n.RootData.mu.Unlock()
	}()

	n.RootData.mu.Lock()

	log.Debugf("Node.Mkdir %s", name)

	path := filepath.Join("/", n.Path(n.Root()), name)
	dir, err := n.RootData.mcfsapi.Mkdir(path)
	if err != nil {
		return nil, syscall.EINVAL
	}

	out.Uid = n.RootData.uid
	out.Gid = n.RootData.gid

	now := time.Now()
	out.SetTimes(&now, &now, &now)

	node := n.newNode()
	return n.NewInode(ctx, node, fs.StableAttr{Mode: n.getMode(dir)}), fs.OK
}

func (n *Node) Rmdir(_ context.Context, name string) syscall.Errno {
	log.Errorf("Node.Rmdir %s name %s", n.Path(n.Root()), name)
	return syscall.EIO
}

// Create will create a new file. At this point the file shouldn't exist. However, because multiple users could be
// uploading files, there is a chance it does exist. If that happens then a new version of the file is created instead.
func (n *Node) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (inode *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	defer func() {
		if r := recover(); r != nil {
			inode = nil
			fh = nil
			fuseFlags = 0
			errno = syscall.EIO
		}
	}()

	fpath := filepath.Join("/", n.Path(n.Root()), name)
	f, err := n.RootData.mcfsapi.Create(fpath)
	if err != nil {
		log.Errorf("Node.Create - failed creating new file %s: %s\n", fpath, err)
		fmt.Printf("Node.Create - failed creating new file %s: %s\n", fpath, err)
		return nil, nil, 0, syscall.EIO
	}

	flags = flags &^ syscall.O_APPEND
	fd, err := syscall.Open(f.ToUnderlyingFilePath(n.RootData.mcfsRoot), int(flags)|os.O_CREATE, mode)
	if err != nil {
		log.Errorf("Node.Create - syscall.Open failed %s: %s\n", f.ToUnderlyingFilePath(n.RootData.mcfsRoot), err)
		return nil, nil, 0, syscall.EIO
	}

	statInfo := syscall.Stat_t{}
	if err := syscall.Fstat(fd, &statInfo); err != nil {
		// TODO - Remove newly created file version in db
		_ = syscall.Close(fd)
		return nil, nil, 0, fs.ToErrno(err)
	}

	node := n.newNode()
	out.FromStat(&statInfo)
	fhandle := n.RootData.newFileHandle(fd, int(flags), fpath, f)
	stableAttr := fs.StableAttr{Mode: n.getMode(f)}
	return n.NewInode(ctx, node, stableAttr), fhandle, 0, fs.OK
}

// Open will open an existing file.
func (n *Node) Open(_ context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	defer func() {
		if r := recover(); r != nil {
			fh = nil
			fmt.Println("Open panicked")
			fuseFlags = 0
			errno = syscall.EIO
		}
	}()

	path := filepath.Join("/", n.Path(n.Root()))
	fmt.Println("Node.Open", path)
	omode := flags & syscall.O_ACCMODE

	f, isNewFile, err := n.RootData.mcfsapi.Open(path, int(flags))
	if err != nil {
		return nil, 0, syscall.EIO
	}

	if omode == syscall.O_WRONLY || omode == syscall.O_RDWR {
		if isNewFile {
			flags = flags &^ syscall.O_CREAT
		}
	}

	filePath := f.ToUnderlyingFilePath(n.RootData.mcfsRoot)
	fd, err := syscall.Open(filePath, int(flags), 0755)
	if err != nil {
		log.Debugf("syscall.Open (%s) %s: %s\n", path, filePath, err)
		return nil, 0, fs.ToErrno(err)
	}

	fhandle := n.RootData.newFileHandle(fd, int(flags), path, f)
	return fhandle, 0, fs.OK
}

func (n *Node) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) (errno syscall.Errno) {
	defer func() {
		if r := recover(); r != nil {
			// If there is a panic then for now say that we don't support this call
			errno = syscall.ENOTSUP
		}
	}()

	path := filepath.Join("/", n.Path(n.Root()))
	log.Debugf("Node.Setattr %s", path)

	if fops, ok := f.(fs.FileSetattrer); ok {
		return fops.Setattr(ctx, in, out)
	}

	// If we are then the file handle is null, so we have to do this by
	// getting paths. This will fail if the file is not known.
	realPath, err := n.RootData.mcfsapi.GetKnownFileRealPath(path)
	if err != nil {
		return syscall.ENOENT
	}

	if sz, ok := in.GetSize(); ok {
		if err := syscall.Truncate(realPath, int64(sz)); err != nil {
			return fs.ToErrno(err)
		}

		st := syscall.Stat_t{}
		if err := syscall.Lstat(realPath, &st); err != nil {
			return fs.ToErrno(err)
		}

		out.FromStat(&st)
		return fs.OK
	}

	return syscall.ENOTSUP
}

func (n *Node) Rename(_ context.Context, _ string, _ fs.InodeEmbedder, _ string, _ uint32) syscall.Errno {
	log.Error("Node.Rename")
	return syscall.EPERM
}

func (n *Node) Unlink(_ context.Context, name string) syscall.Errno {
	log.Errorf("Node.Unlink %s, name %s", n.Path(n.Root()), name)
	return syscall.EPERM
}

func (n *Node) Statfs(_ context.Context, out *fuse.StatfsOut) (errno syscall.Errno) {
	defer func() {
		if r := recover(); r != nil {
			// If there is a panic then for now just return success
			errno = fs.OK
		}
	}()

	s := syscall.Statfs_t{}
	if err := syscall.Statfs(n.RootData.mcfsRoot, &s); err != nil {
		return fs.ToErrno(err)
	}

	out.FromStatfsT(&s)

	return fs.OK
}

// getMode returns the mode for the file. It checks if the underlying mcmodel.File is
// a file or directory entry.
func (n *Node) getMode(entry *mcmodel.File) uint32 {
	if entry == nil {
		return 0755 | uint32(syscall.S_IFDIR)
	}

	if entry.IsDir() {
		return 0755 | uint32(syscall.S_IFDIR)
	}

	return 0644 | uint32(syscall.S_IFREG)
}

// inodeHash creates a new inode id from the file path.
func (n *Node) inodeHash(dirPath string, entry *mcmodel.File) uint64 {
	if entry == nil {
		return 1
	}

	h := fnv.New64a()
	p := filepath.Join(dirPath, entry.FullPath())
	_, _ = h.Write([]byte(p))
	return h.Sum64()
}
