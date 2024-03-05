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
	fmt.Println("MCFile.Readdir")
	if f.mcfile == nil {
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
				UpdatedAt: project.UpdatedAt,
			}
			projectList = append(projectList, f.ToFileInfo())
		}

		return projectList, nil
	}

	return nil, fmt.Errorf("method Readdir not implemented on non-nil mcfile")
}

func (f *MCFile) Stat() (fs.FileInfo, error) {
	if f.mcfile == nil {
		fmt.Println("calling stat on /")
	}

	return nil, fmt.Errorf("method Stat not implemented on MCFile")
}
