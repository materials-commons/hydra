package mcdav

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

type MCFile struct {
	*os.File
	fileStor    stor.FileStor
	projectStor stor.ProjectStor
	mcfile      *mcmodel.File
	user        *mcmodel.User
	hasher      hash.Hash
}

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

	// TODO: Instead call f.fileStor.DoneWritingToFile here. I'll need the conversion stor inorder to do this.
	if err := f.fileStor.UpdateMetadataForFileAndProject(f.mcfile, checksum, size); err != nil {
		// log that we couldn't update the database
	}

	return closeErr
}

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

func (f *MCFile) Readdir(_ int) ([]fs.FileInfo, error) {
	if !f.mcfile.IsDir() {
		return nil, os.ErrInvalid
	}

	if f.mcfile.Path == "/" {
		// When mcfile is nil that means we are reading "/", so present a list of projects
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

	// If we are here then list the project files/directory for the given directory
	pathToUse := f.mcfile.Path
	// if mcfile.ID == 0 then this isn't a real directory, but is either / or /<project-slug>.
	if f.mcfile.ID == 0 {
		// Path is something like /project-slug, so turn this into "/" for the given
		// project. All other real project directory paths will have the correct path.
		// It's only these mcdav made up /project-slug entries that have a path we
		// need to account for and correct.
		pathToUse = "/"
	}

	entries, err := f.fileStor.ListDirectoryByPath(f.mcfile.ProjectID, pathToUse)
	if err != nil {
		return nil, err
	}

	var fileList []os.FileInfo
	for _, entry := range entries {
		fileList = append(fileList, entry.ToFileInfo())
	}

	return fileList, nil
}

func (f *MCFile) Stat() (fs.FileInfo, error) {
	if f.mcfile == nil {
		return nil, fs.ErrInvalid
	}

	return f.mcfile.ToFileInfo(), nil
}
