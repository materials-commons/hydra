package mcbridgefs

import (
	"context"
	"fmt"
	"hash/fnv"
	"mime"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/apex/log"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/bridgefs"
	"gorm.io/gorm"
)

type Node struct {
	file *mcmodel.File
	*bridgefs.BridgeNode
}

func (n *Node) embed() *fs.Inode {
	//TODO implement me
	panic("implement me")
}

func (n *Node) EmbeddedInode() *fs.Inode {
	//TODO implement me
	panic("implement me")
}

var (
	uid, gid             uint32
	mcfsRoot             string
	db                   *gorm.DB
	transferRequest      mcmodel.TransferRequest
	openedFilesTracker   *OpenFilesTracker
	fileStore            stor.FileStor
	transferRequestStore stor.TransferRequestStor
	conversionStore      stor.ConversionStor
)

func init() {
	// Get current user so we can set the uid and gid to use
	u, err := user.Current()
	if err != nil {
		panic(err)
	}
	uid32, _ := strconv.ParseUint(u.Uid, 10, 32)
	gid32, _ := strconv.ParseUint(u.Gid, 10, 32)
	uid = uint32(uid32)
	gid = uint32(gid32)

	// Track any files that this instance writes to/create, so that if another instance does the same
	// each of them will see their versions of the file, rather than intermixing them.
	openedFilesTracker = NewOpenFilesTracker()
}

func CreateFS(fsRoot string, dB *gorm.DB, tr mcmodel.TransferRequest) *Node {
	mcfsRoot = fsRoot
	db = dB
	transferRequest = tr
	fileStore = stor.NewGormFileStor(db, fsRoot)
	conversionStore = stor.NewGormConversionStor(db)
	transferRequestStore = stor.NewGormTransferRequestStor(db, fsRoot)
	return rootNode()
}

func rootNode() *Node {
	bridgeRoot, err := bridgefs.NewBridgeRoot(mcfsRoot, nil, nil)
	if err != nil {
		log.Fatalf("Failed to create root node: %s", err)
	}
	return &Node{
		BridgeNode: bridgeRoot.(*bridgefs.BridgeNode),
	}
}

func (n *Node) newNode() *Node {
	return &Node{
		BridgeNode: bridgefs.NewBridgeNode(n.BridgeNode).(*bridgefs.BridgeNode),
	}
}

// Readdir reads the corresponding directory and returns its entries
func (n *Node) Readdir(_ context.Context) (fs.DirStream, syscall.Errno) {
	// Directories can have a large amount of files. To speed up processing
	// Readdir uses queries that don't retrieve either the underlying directory
	// for a mcmodel.File, or the underlying file for a mcmodel.TransferRequestFile.
	// However, the path for the directory is still needed. This is accessed
	// off of the underlying mcmodel.File by the FullPath() routine which is
	// used the inodeHash() and getMode() methods. To work around this we
	// create a single directory (see dirToUse below), and assign this as the
	// directory for all mcmodel.File entries.
	dirPath := filepath.Join("/", n.Path(n.Root()))
	dirToUse := &mcmodel.File{Path: dirPath}

	dir, err := n.getMCDir("")
	if err != nil {
		return nil, syscall.ENOENT
	}

	files, err := transferRequestStore.ListDirectory(dir, &transferRequest)
	if err != nil {
		return nil, syscall.ENOENT
	}

	filesList := make([]fuse.DirEntry, 0, len(files))
	for _, f := range files {
		f.Directory = dirToUse
		entry := fuse.DirEntry{
			Mode: n.getMode(&f),
			Name: f.Name,
			Ino:  n.inodeHash(&f),
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
// return so we always return a 0 for buffer length and success.
func (n *Node) Getxattr(_ context.Context, _ string, _ []byte) (uint32, syscall.Errno) {
	//fmt.Println("Getxattr")
	return 0, fs.OK
}

// Getattr gets attributes about the file
func (n *Node) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	//fmt.Println("Getattr:", n.mcfsRoot(n.Root()), n.IsDir())

	// Owner is always the process the bridge is running as
	out.Uid = uid
	out.Gid = gid

	if n.IsDir() {
		now := time.Now()
		out.SetTimes(&now, &now, &now)
		return fs.OK
	}

	file, err := fileStore.GetFileByPath(transferRequest.ProjectID, filepath.Join("/", n.Path(n.Root())))
	if err != nil {
		log.Errorf("Getattr: GetFileByPath failed (%s): %s\n", filepath.Join("/", n.Path(n.Root())), err)
		return syscall.ENOENT
	}

	st := syscall.Stat_t{}
	if err := syscall.Lstat(file.ToUnderlyingFilePath(mcfsRoot), &st); err != nil {
		log.Errorf("Getattr: Lstat failed (%s): %s\n", file.ToUnderlyingFilePath(mcfsRoot), err)
		return fs.ToErrno(err)
	}

	out.FromStat(&st)

	return fs.OK
}

// Lookup will return information about the current entry.
func (n *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	path := filepath.Join("/", n.Path(n.Root()), name)
	f, err := fileStore.GetFileByPath(transferRequest.ProjectID, path)
	if err != nil {
		return nil, syscall.ENOENT
	}

	out.Uid = uid
	out.Gid = gid
	if f.IsFile() {
		out.Size = f.Size
	}

	now := time.Now()
	out.SetTimes(&now, &f.UpdatedAt, &now)

	node := n.newNode()
	node.file = f
	return n.NewInode(ctx, node, fs.StableAttr{Mode: n.getMode(f), Ino: n.inodeHash(f)}), fs.OK
}

// getMCDir looks a directory up in the database.
func (n *Node) getMCDir(name string) (*mcmodel.File, error) {
	path := filepath.Join("/", n.Path(n.Root()), name)
	return fileStore.GetDirByPath(transferRequest.ProjectID, path)
}

// Mkdir will create a new directory. If an attempt is made to create an existing directory then it will return
// the existing directory rather than returning an error.
func (n *Node) Mkdir(ctx context.Context, name string, _ uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	path := filepath.Join("/", n.Path(n.Root()), name)
	parent, err := n.getMCDir("")
	if err != nil {
		return nil, syscall.EINVAL
	}

	dir, err := fileStore.CreateDirectory(parent.ID, transferRequest.ProjectID, transferRequest.OwnerID, path, name)

	if err != nil {
		return nil, syscall.EINVAL
	}

	out.Uid = uid
	out.Gid = gid

	now := time.Now()
	out.SetTimes(&now, &now, &now)

	node := n.newNode()
	node.file = dir
	return n.NewInode(ctx, node, fs.StableAttr{Mode: n.getMode(dir), Ino: n.inodeHash(dir)}), fs.OK
}

func (n *Node) Rmdir(_ context.Context, name string) syscall.Errno {
	fmt.Printf("Rmdir %s/%s\n", n.Path(n.Root()), name)
	return syscall.EIO
}

// Create will create a new file. At this point the file shouldn't exist. However, because multiple users could be
// uploading files, there is a chance it does exist. If that happens then a new version of the file is created instead.
func (n *Node) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (inode *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	f, err := n.createNewMCFile(name)
	if err != nil {
		log.Errorf("Create - failed creating new file (%s): %s", name, err)
		return nil, nil, 0, syscall.EIO
	}

	path := filepath.Join("/", n.Path(n.Root()), name)
	openedFilesTracker.Store(path, f)

	flags = flags &^ syscall.O_APPEND
	fd, err := syscall.Open(f.ToUnderlyingFilePath(mcfsRoot), int(flags)|os.O_CREATE, mode)
	if err != nil {
		log.Errorf("    Create - syscall.Open failed:", err)
		return nil, nil, 0, syscall.EIO
	}

	statInfo := syscall.Stat_t{}
	if err := syscall.Fstat(fd, &statInfo); err != nil {
		// TODO - Remove newly created file version in db
		_ = syscall.Close(fd)
		return nil, nil, 0, fs.ToErrno(err)
	}

	node := n.newNode()
	node.file = f
	out.FromStat(&statInfo)
	return n.NewInode(ctx, node, fs.StableAttr{Mode: n.getMode(f), Ino: n.inodeHash(f)}), NewFileHandle(fd, flags, path), 0, fs.OK
}

// Open will open an existing file.
func (n *Node) Open(_ context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	var (
		err     error
		newFile *mcmodel.File
	)
	path := filepath.Join("/", n.Path(n.Root()))

	switch flags & syscall.O_ACCMODE {
	case syscall.O_RDONLY:
		newFile = getFromOpenedFiles(path)
	case syscall.O_WRONLY:
		newFile = getFromOpenedFiles(path)
		if newFile == nil {
			newFile, err = n.createNewMCFileVersion()
			if err != nil {
				// TODO: What error should be returned?
				return nil, 0, syscall.EIO
			}

			openedFilesTracker.Store(path, newFile)
		}
		flags = flags &^ syscall.O_CREAT
		flags = flags &^ syscall.O_APPEND
	case syscall.O_RDWR:
		newFile = getFromOpenedFiles(path)
		if newFile == nil {
			newFile, err = n.createNewMCFileVersion()
			if err != nil {
				// TODO: What error should be returned?
				return nil, 0, syscall.EIO
			}
			openedFilesTracker.Store(path, newFile)
		}
		flags = flags &^ syscall.O_CREAT
		flags = flags &^ syscall.O_APPEND
	default:
		return
	}

	filePath := n.file.ToUnderlyingFilePath(mcfsRoot)
	if newFile != nil {
		filePath = newFile.ToUnderlyingFilePath(mcfsRoot)
	}
	fd, err := syscall.Open(filePath, int(flags), 0)
	if err != nil {
		return nil, 0, fs.ToErrno(err)
	}

	fhandle := NewFileHandle(fd, flags, path)
	return fhandle, 0, fs.OK
}

// Setattr will set attributes on a file. Currently, the only attribute supported is setting the size. This is
// done by calling Ftruncate.
func (n *Node) Setattr(_ context.Context, f fs.FileHandle, in *fuse.SetAttrIn, _ *fuse.AttrOut) syscall.Errno {
	if sz, ok := in.GetSize(); ok {
		fh, ok := f.(*FileHandle)
		if !ok {
			// For now lets return fs.OK, because there doesn't seem to be anything here
			return fs.OK
		}
		return fs.ToErrno(syscall.Ftruncate(fh.Fd, int64(sz)))
	}

	return fs.OK
}

// Release will close the file handle and update meta data about the file in the database
func (n *Node) Release(ctx context.Context, f fs.FileHandle) syscall.Errno {
	bridgeFH, ok := f.(fs.FileReleaser)
	if !ok {
		return syscall.EINVAL
	}

	// Call the underling fileHandle to close the actual file
	if err := bridgeFH.Release(ctx); err != fs.OK {
		return err
	}

	// If the file was opened only for read then there is no meta data that needs to be updated.
	fh := bridgeFH.(*FileHandle)
	if fh.Flags&syscall.O_ACCMODE == syscall.O_RDONLY {
		return fs.OK
	}

	// If we are here then the file was opened with a write flag. In this case we need to update the
	// file size, set this as the current file, and if a new checksum was computed, set the checksum.
	fileToUpdate := n.file
	fpath := filepath.Join("/", n.Path(n.Root()))
	nf := openedFilesTracker.Get(fpath)
	if nf != nil && nf.File != nil {
		fileToUpdate = nf.File
	}

	var (
		size  uint64
		attrs fuse.AttrOut
	)

	fhGetAttr, ok := f.(fs.FileGetattrer)
	if !ok {
		size = 0
	}

	if err := fhGetAttr.Getattr(ctx, &attrs); err == fs.OK {
		size = attrs.Size
	}

	var checksum string
	if nf != nil {
		checksum = fmt.Sprintf("%x", nf.hasher.Sum(nil))
	}

	errno := fs.ToErrno(transferRequestStore.MarkFileReleased(fileToUpdate, checksum, transferRequest.ProjectID, int64(size)))

	// Add to convertible list after marking as released to prevent the condition where the
	// file hasn't been released but is picked up for conversion. This is a very unlikely
	// case, but easy to prevent by releasing then adding to conversions list.
	if fileToUpdate.IsConvertible() {
		if _, err := conversionStore.AddFileToConvert(fileToUpdate); err != nil {
			log.Errorf("Failed adding file to conversion: %d", fileToUpdate.ID)
		}
	}

	return errno
}

// createNewMCFileVersion creates a new file version if there isn't already a version of the file
// file associated with this transfer request instance. It checks the openedFilesTracker to determine
// if a new version has already been created. If a new version was already created then it will return
// that version. Otherwise it will create a new version and add it to the OpenedFilesTracker. In
// addition when a new version is created, the associated on disk directory is created and an empty
// file is written to it.
func (n *Node) createNewMCFileVersion() (*mcmodel.File, error) {
	// First check if there is already a version of this file being written to for this upload context.
	existing := getFromOpenedFiles(filepath.Join("/", n.Path(n.Root()), n.file.Name))
	if existing != nil {
		return existing, nil
	}

	var err error

	// There isn't an existing upload, so create a new one
	newFile := &mcmodel.File{
		ProjectID:   n.file.ProjectID,
		Name:        n.file.Name,
		DirectoryID: n.file.DirectoryID,
		Size:        0,
		Checksum:    "",
		MimeType:    n.file.MimeType,
		OwnerID:     n.file.OwnerID,
		Current:     false,
	}

	newFile, _, err = transferRequestStore.CreateNewFile(newFile, n.file.Directory, &transferRequest)
	if err != nil {
		return nil, err
	}

	// Create the empty file for new version
	f, err := os.OpenFile(newFile.ToUnderlyingFilePath(mcfsRoot), os.O_RDWR|os.O_CREATE, 0755)

	if err != nil {
		log.Errorf("os.OpenFile failed (%s): %s\n", newFile.ToUnderlyingFilePath(mcfsRoot), err)
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return newFile, nil
}

// createNewMCFile will create a new mcmodel.File entry for the directory associated
// with the Node. It will create the directory where the file can be written to.
func (n *Node) createNewMCFile(name string) (*mcmodel.File, error) {
	dir, err := n.getMCDir("")
	if err != nil {
		return nil, err
	}

	file := &mcmodel.File{
		ProjectID:   transferRequest.ProjectID,
		Name:        name,
		DirectoryID: dir.ID,
		Size:        0,
		Checksum:    "",
		MimeType:    getMimeType(name),
		OwnerID:     transferRequest.OwnerID,
		Current:     false,
	}

	f, _, err := transferRequestStore.CreateNewFile(file, dir, &transferRequest)
	return f, err
}

// getMimeType will determine the type of file from its extension. It strips out the extra information
// such as the charset and just returns the underlying type. It returns "unknown" for the mime type if
// the mime package is unable to determine the type.
func getMimeType(name string) string {
	mimeType := mime.TypeByExtension(filepath.Ext(name))
	if mimeType == "" {
		return "unknown"
	}

	semicolon := strings.Index(mimeType, ";")
	if semicolon == -1 {
		return strings.TrimSpace(mimeType)
	}

	return strings.TrimSpace(mimeType[:semicolon])
}

func (n *Node) Rename(_ context.Context, name string, newParent fs.InodeEmbedder, newName string, _ uint32) syscall.Errno {
	fmt.Printf("Rename: %s/%s to %s/%s\n", n.Path(n.Root()), name, newParent.EmbeddedInode().Path(n.Root()), newName)
	fromPath := filepath.Join("/", n.Path(n.Root()))
	toPath := filepath.Join("/", newParent.EmbeddedInode().Path(n.Root()))

	dir, err := n.getMCDir("")
	if err != nil {
		return syscall.ENOENT
	}

	var f mcmodel.File
	err = db.Preload("Directory").
		Where("directory_id = ?", dir.ID).
		Where("project_id = ?", transferRequest.ProjectID).
		Where("name = ?", name).
		Where("current = ?", true).
		Where("deleted_at IS NULL").
		Find(&f).Error

	switch {
	case err != nil:
		return syscall.ENOENT
	case f.IsDir():
		return n.renameDir(fromPath, toPath, name, newName, f)
	default:
		// f is a file
		return n.renameFile(fromPath, toPath, name, newName, f)
	}
}

func (n *Node) renameDir(fromPath, toPath, name, toName string, f mcmodel.File) syscall.Errno {
	if fromPath == toPath {
		// not being moved to another directory. Just rename directory and all descendant directory
		// paths
	}
	return syscall.EPERM
}

func (n *Node) renameFile(fromPath, toPath, name, toName string, f mcmodel.File) syscall.Errno {
	if fromPath == toPath {
		// not being moved to another directory. Just rename file and all its previous versions
	}
	return syscall.EPERM
}

func (n *Node) Unlink(_ context.Context, name string) syscall.Errno {
	fmt.Printf("Unlink: %s/%s\n", n.Path(n.Root()), name)
	return syscall.EPERM
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
func (n *Node) inodeHash(entry *mcmodel.File) uint64 {
	if entry == nil {
		return 1
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(entry.FullPath()))
	return h.Sum64()
}

// getFromOpenedFiles returns the mcmodel.File from the openedFilesTracker. It handles
// the case where the path wasn't found.
func getFromOpenedFiles(path string) *mcmodel.File {
	val := openedFilesTracker.Get(path)
	if val != nil {
		return val.File
	}

	return nil
}
