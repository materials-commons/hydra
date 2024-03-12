package mcdav

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
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

// A UserFS represents a virtual fs to a users files and projects. It uses the Materials Commons
// file and project stors to determine projects, files and access. A UserFS is represented in
// a path as /<project-slug>/<rest-of-path>, where <project-slug> is the unique slug for a given
// project. For example /aging-1234/mydir/file.txt represents the path /mydir/file.txt in the
// project identified by the slug aging-1234.
type UserFS struct {
	fileStor       stor.FileStor
	projectStor    stor.ProjectStor
	conversionStor stor.ConversionStor

	// A cache of projects the user has access to
	projects sync.Map

	// projects that the user tried to access that they don't have access to
	projectsWithoutAccess sync.Map

	// MC User this is associated with
	user *mcmodel.User

	// Directory path to mcfs files
	mcfsRoot string

	useKnownFiles bool

	// knownFiles is a list of files (*mcmodel.File) that the system has created. When the user writes to a file
	// they will write to the underlying file represented by this file.
	knownFiles sync.Map
}

// UserFSOpts are the arguments for creating a new UserFS.
type UserFSOpts struct {
	MCFSRoot       string
	User           *mcmodel.User
	ProjectStor    stor.ProjectStor
	FileStor       stor.FileStor
	ConversionStor stor.ConversionStor
}

// NewUserFS creates a new UserFS. All the fields in UserFSOpts must be filled in. This is not checked.
func NewUserFS(opts *UserFSOpts) *UserFS {
	return &UserFS{
		mcfsRoot:       opts.MCFSRoot,
		user:           opts.User,
		projectStor:    opts.ProjectStor,
		fileStor:       opts.FileStor,
		conversionStor: opts.ConversionStor,
		useKnownFiles:  false,
	}
}

// Mkdir creates a new directory. It determines the project and path from the path.
func (fs *UserFS) Mkdir(ctx context.Context, path string, perm os.FileMode) error {
	//fmt.Println("fs.Mkdir", path)
	project, projectSlug, err := fs.getProject(path)
	if err != nil {
		return err
	}

	dirPath := mc.RemoveProjectSlugFromPath(path, projectSlug)
	parentDir, err := fs.fileStor.GetFileByPath(project.ID, filepath.Dir(dirPath))
	if err != nil {
		return err
	}

	_, err = fs.fileStor.CreateDirectory(parentDir.ID, project.ID, fs.user.ID, dirPath, filepath.Base(dirPath))

	return err
}

// OpenFile opens a file project file. It also handles virtual objects that don't actually but are represented
// in the path. For example "/", which is the base directory, and can be used by WebDAV to get a list of all
// the project slugs (represented as directories) for a given user. The same for /<project-slug> which
// represents a particular project with <project-slug> as its slug identifier.
func (fs *UserFS) OpenFile(ctx context.Context, path string, flags int, perm os.FileMode) (webdav.File, error) {
	//log.Infof("fs.OpenFile(%s, %o)\n", path, flags)

	if path == "/" {
		// Listing root, create a fake mcmodel.File object to represent this. MCFile will see this
		// and known to list projects for the user when Readdir() is called.
		f := &mcmodel.File{
			MimeType:  "directory",
			Name:      "/",
			Size:      0,
			UpdatedAt: time.Now(),
			Path:      "/",
		}
		mcfile := &MCFile{
			File:           nil, // There is no real underlying File to open.
			fileStor:       fs.fileStor,
			projectStor:    fs.projectStor,
			conversionStor: fs.conversionStor,
			mcfile:         f,
			user:           fs.user,
		}

		return mcfile, nil
	}

	// At this point the path looks like /<project-slug>/<... rest of path ...>. The means the path
	// represents a real project through the project slug.

	project, projectSlug, err := fs.getProject(path)
	if err != nil {
		return nil, err
	}

	// Check if path is only /<project-slug/, and if so create a fake entry to represent this.
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
			File:           nil,
			fileStor:       fs.fileStor,
			projectStor:    fs.projectStor,
			conversionStor: fs.conversionStor,
			mcfile:         f,
			user:           fs.user,
		}

		return mcfile, nil
	}

	// If we are here then the path points to a real project file.

	// Remove the projectSlug from the path to get to the project path.
	filePath := mc.RemoveProjectSlugFromPath(path, projectSlug)

	// We are going to attempt to find the file. There are a few different cases to handle, they are
	//   1. OpenFile on an existing file for read/write
	//          If this happens we check if there is a previous reference in knownFiles, if not create one,
	//          otherwise return the one we found. Also open or create the underlying file.
	//   2. OpenFile on an existing file for read-only
	//          If the file can't be found this is an error, since it doesn't make sense to create a non-existing
	//          file for read-only access.
	//   3. OpenFile on an existing directory.
	//          If this can't be found, then this is an error, as Mkdir is a different call. Otherwise, create
	//          directory entry MCFile and return it.

	file, err := fs.fileStor.GetFileByPath(project.ID, filePath)
	if err != nil {
		// File not found. If this is to read the file, then it's an error, otherwise we need
		// to create the file.
		if isReadonly(flags) {
			return nil, err
		}

		// Couldn't find the file, so create a new one.
		return fs.createFile(filePath, project)
	}

	// found file, check if it's a directory reference

	if file.IsDir() {
		mcfile := &MCFile{
			File:           nil,
			fileStor:       fs.fileStor,
			projectStor:    fs.projectStor,
			conversionStor: fs.conversionStor,
			mcfile:         file,
			user:           fs.user,
		}

		return mcfile, nil
	}

	// If we are here then the file exists. For readonly we just open the file.
	if isReadonly(flags) {
		return os.Open(file.ToUnderlyingFilePath(fs.mcfsRoot))
	}

	// For all other cases we create the file (or return the file from knownFiles).
	return fs.createFile(filePath, project)
}

// createFile checks if a file entry was already accessed in knownFiles. If so it uses that. Otherwise,
// it creates a new version, sticks it in knownFiles and uses it.
func (fs *UserFS) createFile(filePath string, project *mcmodel.Project) (webdav.File, error) {
	if fs.useKnownFiles {
		knownFile, ok := fs.knownFiles.Load(filePath)
		if ok {
			// This file has already been created.
			file := knownFile.(*mcmodel.File)

			// Always open with O_TRUNC, because WebDAV will resend the entire file contents.
			f, err := os.OpenFile(file.ToUnderlyingFilePath(fs.mcfsRoot), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
			if err != nil {
				return nil, err
			}

			mcfile := &MCFile{
				File:           f,
				fileStor:       fs.fileStor,
				projectStor:    fs.projectStor,
				conversionStor: fs.conversionStor,
				mcfile:         file,
				user:           fs.user,
				hasher:         md5.New(),
			}

			return mcfile, nil
		}
	}

	// if we are here then there wasn't an entry in knownFiles, so we need to create a new file in the project, and
	// stick the newly created file in knownFiles.

	dirPath := filepath.Dir(filePath)

	// Find the directory entry where the file will reside.
	dir, err := fs.fileStor.GetDirByPath(project.ID, dirPath)
	if err != nil {
		return nil, err
	}

	// Create the file in the given directory.
	name := filepath.Base(filePath)
	file, err := fs.fileStor.CreateFile(name, project.ID, dir.ID, fs.user.ID, mc.GetMimeType(name))
	if err != nil {
		return nil, err
	}

	// Make newly created file as current, setting previous versions as not-current.
	file, err = fs.fileStor.SetFileAsCurrent(file)

	// Create the underlying directory path from the UUID.
	err = file.MkdirUnderlyingPath(fs.mcfsRoot)
	if err != nil {
		// What to do with the already created database file entry?
		return nil, err
	}

	// Create an empty file
	f, err := os.Create(file.ToUnderlyingFilePath(fs.mcfsRoot))
	if err != nil {
		// What to do with the already created database file entry?
		return nil, err
	}

	mcfile := &MCFile{
		File:           f,
		fileStor:       fs.fileStor,
		projectStor:    fs.projectStor,
		conversionStor: fs.conversionStor,
		mcfile:         file,
		user:           fs.user,
		hasher:         md5.New(),
	}

	// place in knownFiles so subsequent attempts to write will reuse this entry.
	fs.knownFiles.Store(filePath, file)

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

// Stat get the stat (os.FileInfo) for a file. It handles "/" and /<project-slug>
// paths by creating fake entries for them.
func (fs *UserFS) Stat(ctx context.Context, path string) (os.FileInfo, error) {
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
		// Path is /<project-slug> create a fake entry representing the project.
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
