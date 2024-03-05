package mcdav

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/apex/log"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcssh/mc"
	"github.com/materials-commons/hydra/pkg/webdav"
)

type UserFS struct {
	fileStor    stor.FileStor
	projectStor stor.ProjectStor

	// A cache of projects the user has access to
	projects sync.Map

	// projects that the user tried to access that they don't have access to
	projectsWithoutAccess sync.Map

	// MC User this is associated with
	user *mcmodel.User

	// Directory path to mcfs files
	mcfsRoot string

	// filesWritten keeps track of files that the user has written to. Because
	// we don't know when a user is done writing to a file, only the first time
	// a write() is done to a file do we create a new version. After that, all
	// subsequent writes are done to the same file. This map can be reset by
	// a user from the UI, or the CLI when they know they want a new version.
	filesWritten sync.Map
}

// slashClean is equivalent to but slightly more efficient than
// path.Clean("/" + name).
// From Golang source for x/net/webdav package
func slashClean(name string) string {
	if name == "" || name[0] != '/' {
		name = "/" + name
	}
	return path.Clean(name)
}

type UserFSOpts struct {
	MCFSRoot    string
	User        *mcmodel.User
	ProjectStor stor.ProjectStor
	FileStor    stor.FileStor
}

func NewUserFS(opts *UserFSOpts) *UserFS {
	return &UserFS{
		mcfsRoot:    opts.MCFSRoot,
		user:        opts.User,
		projectStor: opts.ProjectStor,
		fileStor:    opts.FileStor,
	}
}

func (fs *UserFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	fmt.Println("fs.Mkdir", name)
	return fmt.Errorf("method Mkdir not implemented")
}

func (fs *UserFS) OpenFile(ctx context.Context, path string, flag int, perm os.FileMode) (webdav.File, error) {
	fmt.Println("fs.OpenFile", path)

	if path == "/" {
		// Listing root
		f := &mcmodel.File{
			MimeType:  "directory",
			Name:      "/",
			Size:      0,
			UpdatedAt: time.Now(),
			Path:      "/",
		}
		mcfile := &MCFile{
			File:        nil,
			fileStor:    fs.fileStor,
			projectStor: fs.projectStor,
			mcfile:      f,
			user:        fs.user,
		}

		return mcfile, nil
	}

	project, projectSlug, err := fs.getProject(path)
	if err != nil {
		return nil, err
	}

	if pathIsOnlyForProjectSlug(path) {
		f := &mcmodel.File{
			MimeType:  "directory",
			Name:      projectSlug,
			Size:      0,
			UpdatedAt: time.Now(),
			Path:      path,
			ProjectID: project.ID,
		}

		mcfile := &MCFile{
			File:        nil,
			fileStor:    fs.fileStor,
			projectStor: fs.projectStor,
			mcfile:      f,
			user:        fs.user,
		}

		return mcfile, nil
	}

	// If we are here then this is an open on a file.

	filePath := mc.RemoveProjectSlugFromPath(path, projectSlug)

	file, err := fs.fileStor.GetFileByPath(project.ID, filePath)
	if err != nil {
		// File not found. If this is to read the file, then it's an error, otherwise we need
		// to create the file.
		if isReadonly(flag) {
			return nil, err
		}

		// if we are here then the lookup failed, but this is for a write() to the file, so create
		// a new one.
		dirPath := filepath.Dir(filePath)

		dir, err := fs.fileStor.GetDirByPath(project.ID, dirPath)
		if err != nil {
			return nil, err
		}

		name := filepath.Base(filePath)
		file, err = fs.fileStor.CreateFile(name, project.ID, dir.ID, fs.user.ID, mc.GetMimeType(name))
		if err != nil {
			return nil, err
		}

		err = file.MkdirUnderlyingPath(fs.mcfsRoot)
		if err != nil {
			// What to do with the already created database file entry?
			return nil, err
		}

		// Create empty file
		f, err := os.Create(file.ToUnderlyingFilePath(fs.mcfsRoot))
		if err != nil {
			// What to do with the already created database file entry?
			return nil, err
		}

		mcfile := &MCFile{
			File:        f,
			fileStor:    fs.fileStor,
			projectStor: fs.projectStor,
			mcfile:      file,
			user:        fs.user,
		}

		return mcfile, nil
	}

	if file.IsDir() {
		mcfile := &MCFile{
			File:        nil,
			fileStor:    fs.fileStor,
			projectStor: fs.projectStor,
			mcfile:      file,
			user:        fs.user,
		}

		return mcfile, nil
	}

	// If we are here then the file exists, so open it for either read or write.
	if isReadonly(flag) {
		return os.Open(file.ToUnderlyingFilePath(fs.mcfsRoot))
	}

	f, err := os.OpenFile(file.ToUnderlyingFilePath(fs.mcfsRoot), flag, 0777)
	if err != nil {
		return nil, err
	}

	mcfile := &MCFile{
		File:        f,
		fileStor:    fs.fileStor,
		projectStor: fs.projectStor,
		mcfile:      file,
		user:        fs.user,
	}

	return mcfile, nil
}

func (fs *UserFS) RemoveAll(ctx context.Context, name string) error {
	fmt.Println("fs.RemoveAll", name)
	return fmt.Errorf("method RemoveAll not implemented")
}

func (fs *UserFS) Rename(ctx context.Context, oldName, newName string) error {
	fmt.Println("fs.Rename", oldName, newName)
	return fmt.Errorf("method Rename not implemented")
}

func (fs *UserFS) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	fmt.Println("fs.Stat", path)
	if path == "/" {
		// Listing root. Create a fake entry.
		f := mcmodel.File{
			MimeType:  "directory",
			Name:      "/",
			Size:      0,
			UpdatedAt: time.Now(),
			Path:      "/",
		}
		return f.ToFileInfo(), nil
	}

	project, projectSlug, err := fs.getProject(path)
	if err != nil {
		return nil, err
	}

	if pathIsOnlyForProjectSlug(path) {
		f := mcmodel.File{
			MimeType:  "directory",
			Name:      projectSlug,
			Size:      0,
			UpdatedAt: time.Now(),
			Path:      path,
			ProjectID: project.ID,
		}

		return f.ToFileInfo(), nil
	}

	// If we are here then the stat is on a path like the following:
	//  /<proj-slug>/..., for example, given project slug proj1
	//  /proj1/dir/something.txt
	// The stat is on /dir/something.txt in project with slug proj1
	//
	projectPath := mc.RemoveProjectSlugFromPath(path, projectSlug)
	f, err := fs.fileStor.GetFileByPath(project.ID, projectPath)
	if err != nil {
		return nil, err
	}

	return f.ToFileInfo(), nil
}

// getProject retrieves the project from the path. The r.Filepath contains the project slug as
// a part of the path. This method strips that out. The mcfsHandler has two caches for projects
// the first mcfsHandler.projects is a cache of already loaded projects, indexed by the slug. The
// second is mcfsHandler.projectsWithoutAccess which is a cache of booleans indexed by the project
// slug for project slugs that either don't exist or that the user doesn't have access to. Only
// if the slug isn't found in either of these caches is an attempt to look it up (and if the
// lookup is successful also check access) done. The lookup will fill out the appropriate
// project cache (mcfsHandler.projects or mcfsHandler.projectsWithoutAccess).
//
// TODO: This code is copied from the mcsftp.Handler. Refactor so both share a common base
func (fs *UserFS) getProject(path string) (*mcmodel.Project, string, error) {
	projectSlug := mc.GetProjectSlugFromPath(path)

	// Check if we previously found this project.
	if proj, ok := fs.projects.Load(projectSlug); ok {
		// Paranoid check - Make sure that the item returned is a *mcmodel.Project
		// and return an error if it isn't.
		p, okCast := proj.(*mcmodel.Project)
		if !okCast {
			// Bug - The item stored in h.projects is not a *mcmodel.Project, so delete
			// it and return an error saying we can't find the project. Also set the
			// projectSlug in h.projectsWithoutAccess so, we don't just continually try
			// to load this.
			fs.projects.Delete(projectSlug)
			fs.projectsWithoutAccess.Store(projectSlug, true)
			log.Errorf("error casting to project for slug %s", projectSlug)
			return nil, "", fmt.Errorf("no such project: %s", projectSlug)
		}

		return p, projectSlug, nil
	}

	// Check if we tried to load the project in the past and failed.
	if _, ok := fs.projectsWithoutAccess.Load(projectSlug); ok {
		return nil, "", fmt.Errorf("no such project: %s", projectSlug)
	}

	// If we are here then we've never tried loading the project.

	var (
		project *mcmodel.Project
		err     error
	)

	if project, err = mc.GetAndValidateProjectFromPath(path, fs.user.ID, fs.projectStor); err != nil {
		// Error looking up or validating access. Mark this project slug as invalid.
		fs.projectsWithoutAccess.Store(projectSlug, true)
		return nil, "", err
	}

	// Found the project and user has access so put in the projects cache.
	fs.projects.Store(projectSlug, project)

	return project, projectSlug, nil
}

// TODO flagSet and isReadonly are from mcfs. Move these into a fsutil directory or something to share
// between packages.

func flagSet(flags, flagToCheck int) bool {
	return flags&flagToCheck == flagToCheck
}

func isReadonly(flags int) bool {
	switch {
	case flagSet(flags, syscall.O_WRONLY):
		return false
	case flagSet(flags, syscall.O_RDWR):
		return false
	default:
		// For open one of O_WRONLY, O_RDWR or O_RDONLY must be set. If
		// we are here then neither O_WRONLY nor O_RDWR was set, so O_RDONLY
		// must be true.
		return true
	}
}
