package mcfs

import (
	"mime"
	"path/filepath"
	"strings"

	"github.com/materials-commons/hydra/pkg/mcbridgefs/fs/mcfs/projectpath"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

// MCFSApi is the file system interface into Materials Commons. It has little knowledge of
// FUSE. It understands the Materials Commons calls to make to achieve certain file system
// operations, and returns the results in a way that the node can pass back.
type MCFSApi struct {
	fileStor            stor.FileStor
	transferRequestStor stor.TransferRequestStor
	conversionStor      stor.ConversionStor
	mcfsRoot            string
}

func NewMCFSApi() *MCFSApi {
	return nil
}

func (fs *MCFSApi) Readdir(path string) ([]mcmodel.File, error) {
	projPath := projectpath.NewProjectPath(path)

	dir, err := fs.fileStor.GetDirByPath(projPath.ProjectID, projPath.ProjectPath)
	if err != nil {
		return nil, err
	}

	transferRequest, err := fs.transferRequestStor.GetTransferRequestByProjectAndUser(projPath.ProjectID, projPath.UserID)
	if err != nil {
		return nil, err
	}

	// Make list directory to a pointer for transferRequest?
	dirEntries, err := fs.transferRequestStor.ListDirectory(dir, transferRequest)

	inDir := &mcmodel.File{Path: projPath.ProjectPath}
	for _, entry := range dirEntries {
		entry.Directory = inDir
	}

	return dirEntries, nil
}

func (fs *MCFSApi) GetRealPath(path string) (realpath string, err error) {
	projPath := projectpath.NewProjectPath(path)
	file, err := fs.fileStor.GetFileByPath(projPath.ProjectID, projPath.ProjectPath)
	if err != nil {
		return "", err
	}

	return file.ToUnderlyingFilePath(fs.mcfsRoot), nil
}

func (fs *MCFSApi) Lookup(path string) (*mcmodel.File, error) {
	projPath := projectpath.NewProjectPath(path)
	f, err := fs.fileStor.GetFileByPath(projPath.ProjectID, projPath.ProjectPath)

	return f, err
}

func (fs *MCFSApi) Mkdir(path string) (*mcmodel.File, error) {
	projPath := projectpath.NewProjectPath(path)
	parentDir, err := fs.fileStor.GetFileByPath(projPath.ProjectID, filepath.Dir(projPath.ProjectPath))
	if err != nil {
		return nil, err
	}

	return fs.fileStor.CreateDirectory(parentDir.ID, projPath.ProjectID, projPath.UserID, projPath.ProjectPath, filepath.Base(projPath.ProjectPath))
}

func (fs *MCFSApi) Create(path string) (*mcmodel.File, error) {
	projPath := projectpath.NewProjectPath(path)
	f, err := fs.createNewFile(projPath.ProjectID, filepath.Dir(projPath.ProjectPath), filepath.Base(projPath.ProjectPath))

	return f, err
}

func (fs *MCFSApi) Open() {

}

// Release Move out of the file handle?
func (fs *MCFSApi) Release() {

}

func (fs *MCFSApi) createNewFile(projectID int, dirPath string, name string) (*mcmodel.File, error) {
	dir, err := fs.fileStor.GetDirByPath(projectID, dirPath)
	if err != nil {
		return nil, err
	}

	file := &mcmodel.File{
		ProjectID:   projectID,
		Name:        name,
		DirectoryID: dir.ID,
		Size:        0,
		Checksum:    "",
		MimeType:    determineMimeType(name),
		OwnerID:     transferRequest.OwnerID,
		Current:     false,
	}

	return fs.transferRequestStor.CreateNewFile(file, dir, transferRequest)
}

// determineMimeType ... Move this into a utility package.
func determineMimeType(name string) string {
	mimeType := mime.TypeByExtension(filepath.Ext(name))
	if mimeType == "" {
		return "unknown"
	}

	mediatype, _, err := mime.ParseMediaType(mimeType)
	if err != nil {
		// ParseMediaType returned an error, but TypeByExtension
		// returned a mime string. As a fallback let's remove
		// any parameters on the string (if there is a semicolon
		// it will be after the semicolon), and return everything
		// before the (optional) semicolon.
		semicolon := strings.Index(mimeType, ";")
		if semicolon == -1 {
			return strings.TrimSpace(mimeType)
		}

		return strings.TrimSpace(mimeType[:semicolon])
	}

	return strings.TrimSpace(mediatype)
}
