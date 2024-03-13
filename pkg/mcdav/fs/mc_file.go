package fs

import (
	"bytes"
	"context"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

// MCFile represents the underlying "/" (root), project, or file/directory in a project.
// Only a real project file (as opposed to a directory) will have a non-nil os.File.
type MCFile struct {
	// If there is a real file, then this will be non-nil
	*os.File
	fileStor       stor.FileStor
	projectStor    stor.ProjectStor
	conversionStor stor.ConversionStor

	// The materials commons file entry, or a fake on for a project or "/"
	mcfile *mcmodel.File

	// The Materials Commons user for this entry. This determines, for project
	// and "/" what to show.
	user *mcmodel.User

	// When a real file is opened, each write updates the checksum we are accumulating.
	hasher hash.Hash
}

// Close will close the underlying os.File if its non-nil. Note that readonly files
// from UserFS.OpenFile only return a *os.File and not a MCFile. That means that
// close doesn't need to handle the case where the actually opened file is a file
// that was only read and not written.
func (f *MCFile) Close() error {
	if f.File == nil {
		return nil
	}

	// All MCFile entries are files that were opened for write. So lets update the metadata.

	var size int64 = 0
	finfo, err := f.File.Stat()
	if err == nil {
		size = finfo.Size()
	}

	closeErr := f.File.Close()

	checksum := fmt.Sprintf("%x", f.hasher.Sum(nil))

	if err := f.fileStor.UpdateMetadataForFileAndProject(f.mcfile, checksum, size); err != nil {
		// log that we couldn't update the database
	}

	if f.mcfile.IsConvertible() {
		if _, err := f.conversionStor.AddFileToConvert(f.mcfile); err != nil {
			// log that couldn't update the database.
		}
	}

	return closeErr
}

// Write to the file and update the checksum (hasher) state.
func (f *MCFile) Write(p []byte) (n int, err error) {
	if f.File == nil {
		return 0, os.ErrInvalid
	}

	if n, err = f.File.Write(p); err != nil {
		return 0, err
	}

	_, _ = io.Copy(f.hasher, bytes.NewBuffer(p[:n]))

	return n, err
}

func (f *MCFile) ContentType(ctx context.Context) (string, error) {
	return f.mcfile.MimeType, nil
}

// Readdir returns the list of files or projects for an MCFile.
func (f *MCFile) Readdir(_ int) ([]fs.FileInfo, error) {
	if !f.mcfile.IsDir() {
		return nil, os.ErrInvalid
	}

	if f.mcfile.Path == "/" {
		// The path is "/" so list the users projects as directories.
		projects, err := f.projectStor.GetProjectsForUser(f.user.ID)
		if err != nil {
			return nil, err
		}
		var projectList []os.FileInfo

		// Go through each project creating a fake file (directory) that is the project slug
		for _, project := range projects {
			f := mcmodel.File{
				Name:      project.Slug,
				MimeType:  "directory",
				Size:      uint64(project.Size),
				Path:      filepath.Join("/", project.Slug),
				ProjectID: project.ID,
				UpdatedAt: project.UpdatedAt,
			}
			projectList = append(projectList, f.ToFileInfo())
		}

		return projectList, nil
	}

	// If we are here then list the project files/directory for the given directory.
	// f.mcfile.Path contains a project specific path (so no project-slug)
	pathToUse := f.mcfile.Path

	if f.mcfile.ID == 0 {
		// if mcfile.ID == 0 then this isn't a real directory, since "/" was handled above
		// it must be a directory listing for a /<project-slug>.
		//
		// When we do the lookup we are asking for the files/directories in that
		// projects root. So change pathToUse to "/".
		pathToUse = "/"
	}

	entries, err := f.fileStor.ListDirectoryByPath(f.mcfile.ProjectID, pathToUse)
	if err != nil {
		return nil, err
	}

	// Convert entries to fs.FileInfo
	var fileList []os.FileInfo
	for _, entry := range entries {
		fileList = append(fileList, entry.ToFileInfo())
	}

	return fileList, nil
}

// Stat returns the mcfile info as a fs.FileInfo.
func (f *MCFile) Stat() (fs.FileInfo, error) {
	if f.mcfile == nil {
		return nil, fs.ErrInvalid
	}

	return f.mcfile.ToFileInfo(), nil
}
