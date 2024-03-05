package mcdav

import (
	"context"
	"fmt"
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
}

func (f *MCFile) Close() error {
	if f.File == nil {
		return nil
	}
	err := f.File.Close()
	// Update metadata in materials commons
	return err
}

func (f *MCFile) Write(p []byte) (n int, err error) {
	// write to hash
	return f.File.Write(p)
}

func (f *MCFile) ContentType(ctx context.Context) (string, error) {
	fmt.Println("Calling MCFile.ContentType")
	return f.mcfile.MimeType, nil
}

func (f *MCFile) Readdir(_ int) ([]fs.FileInfo, error) {
	fmt.Printf("MCFile.Readdir %#v\n", f.mcfile)
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
	if pathIsOnlyForProjectSlug(pathToUse) {
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
