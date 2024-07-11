package handler

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcssh/mc"
	"github.com/materials-commons/hydra/pkg/uid"
	"github.com/tus/tusd/v2/pkg/handler"
	"gorm.io/gorm"
)

type MCFileStore struct {
	db      *gorm.DB
	mcfsDir string
}

func NewMCFileStore(db *gorm.DB, mcfsDir string) *MCFileStore {
	return &MCFileStore{
		db:      db,
		mcfsDir: mcfsDir,
	}
}

// NewUpload creates a new upload using the size as the file's length. The method must
// return a unique id which is used to identify the upload. If no backend
// (e.g. Riak) specifes the id you may want to use the uid package to
// generate one. The properties Size and MetaData will be filled.
func (s *MCFileStore) NewUpload(ctx context.Context, info handler.FileInfo) (upload handler.Upload, err error) {
	filename := info.MetaData["filename"]
	projectID, _ := strconv.Atoi(info.MetaData["project_id"])
	directoryID, _ := strconv.Atoi(info.MetaData["directory_id"])
	ownerID, _ := strconv.Atoi(info.MetaData["owner_id"])

	if info.ID == "" {
		info.ID = uid.Uid()
	}

	fileUpload := &MCFileUpload{
		fileStor:       stor.NewGormFileStor(s.db, s.mcfsDir),
		conversionStor: stor.NewGormConversionStor(s.db),
		MCFSDir:        s.mcfsDir,
		FileInfo:       info,
		checksum:       "",
		Filename:       filename,
		ProjectID:      projectID,
		DirectoryID:    directoryID,
		OwnerID:        ownerID,
	}

	if err := createFile(fileUpload.getChunkPath(), nil); err != nil {
		return nil, err
	}

	if err := fileUpload.saveState(); err != nil {
		return nil, err
	}

	return fileUpload, nil
}

// GetUpload fetches the upload with a given ID. If no such upload can be found,
// ErrNotFound must be returned.
func (s *MCFileStore) GetUpload(ctx context.Context, id string) (upload handler.Upload, err error) {
	var mcFileUpload MCFileUpload

	data, err := os.ReadFile(getStatePathByID(s.mcfsDir, id))
	if err != nil {
		if os.IsNotExist(err) {
			err = handler.ErrNotFound
		}
		return nil, err
	}

	if err := json.Unmarshal(data, &mcFileUpload); err != nil {
		return nil, err
	}

	finfo, err := os.Stat(getChunkPathByID(s.mcfsDir, id))
	if err != nil {
		if os.IsNotExist(err) {
			err = handler.ErrNotFound
		}

		return nil, err
	}

	mcFileUpload.FileInfo.Size = finfo.Size()
	mcFileUpload.fileStor = stor.NewGormFileStor(s.db, s.mcfsDir)
	mcFileUpload.conversionStor = stor.NewGormConversionStor(s.db)

	return &mcFileUpload, nil
}

func (s *MCFileStore) UseIn(composer *handler.StoreComposer) {
	composer.UseCore(s)
	composer.UseTerminater(s)
	composer.UseConcater(s)
	composer.UseLengthDeferrer(s)
}

func (s *MCFileStore) AsTerminatableUpload(upload handler.Upload) handler.TerminatableUpload {
	return upload.(*MCFileUpload)
}

func (s *MCFileStore) AsLengthDeclarableUpload(upload handler.Upload) handler.LengthDeclarableUpload {
	return upload.(*MCFileUpload)
}

func (s *MCFileStore) AsConcatableUpload(upload handler.Upload) handler.ConcatableUpload {
	return upload.(*MCFileUpload)
}

type MCFileUpload struct {
	fileStor       stor.FileStor
	conversionStor stor.ConversionStor
	MCFSDir        string
	FileInfo       handler.FileInfo
	checksum       string
	ProjectID      int
	DirectoryID    int
	OwnerID        int
	Filename       string
}

func (u *MCFileUpload) GetInfo(ctx context.Context) (info handler.FileInfo, err error) {
	return u.FileInfo, nil
}

func (u *MCFileUpload) WriteChunk(ctx context.Context, offset int64, src io.Reader) (int64, error) {
	file, err := os.OpenFile(u.getChunkPath(), os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return 0, err
	}
	// Avoid the use of defer file.Close() here to ensure no errors are lost
	// See https://github.com/tus/tusd/issues/698.

	n, err := io.Copy(file, src)
	u.FileInfo.Offset += n
	if err != nil {
		_ = file.Close()
		return n, err
	}

	return n, file.Close()

}

func (u *MCFileUpload) GetReader(ctx context.Context) (io.ReadCloser, error) {
	return os.Open(u.getChunkPath())
}

func (u *MCFileUpload) Terminate(ctx context.Context) error {
	_ = os.Remove(u.getChunkPath())
	_ = os.Remove(u.getStatePath())
	return nil
}

func (u *MCFileUpload) ConcatUploads(ctx context.Context, uploads []handler.Upload) (err error) {
	// Create the hasher and start it by reading the first chunk that we will be appending to
	hasher := md5.New()
	d, err := os.ReadFile(u.getChunkPath())
	if err != nil {
		return err
	}
	_, err = io.Copy(hasher, bytes.NewReader(d))
	if err != nil {
		return err
	}

	// Now that the hash is started, we can reopen the first chunk and start appending
	// the other chunks to it.

	file, err := os.OpenFile(u.getChunkPath(), os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	defer func() {
		// Ensure that close error is propagated, if it occurs.
		// See https://github.com/tus/tusd/issues/698.
		cerr := file.Close()
		if err == nil {
			err = cerr
		}
	}()

	for _, partialUpload := range uploads {
		fileUpload := partialUpload.(*MCFileUpload)
		src, err := os.Open(fileUpload.getChunkPath())
		if err != nil {
			return err
		}

		teeReader := io.TeeReader(src, hasher)

		if _, err := io.Copy(file, teeReader); err != nil {
			_ = src.Close()
			return err
		}

		_ = os.Remove(fileUpload.getChunkPath())
		_ = os.Remove(fileUpload.getStatePath())
		_ = src.Close()
	}

	u.checksum = fmt.Sprintf("%x", hasher.Sum(nil))

	return nil
}

func (u *MCFileUpload) getChunkPath() string {
	return getChunkPathByID(u.MCFSDir, u.FileInfo.ID)
}

func (u *MCFileUpload) getStatePath() string {
	return getStatePathByID(u.MCFSDir, u.FileInfo.ID)
}

func (u *MCFileUpload) DeclareLength(ctx context.Context, length int64) error {
	u.FileInfo.Size = length
	u.FileInfo.SizeIsDeferred = false
	return u.saveState()
}

func (u *MCFileUpload) FinishUpload(ctx context.Context) error {
	if u.FileInfo.IsFinal {
		// If checksum is "" then there was a single chunk, and we need to compute the checksum here.
		if u.checksum == "" {
			hasher := md5.New()
			d, err := os.ReadFile(u.getChunkPath())
			if err != nil {
				return err
			}
			_, err = io.Copy(hasher, bytes.NewReader(d))
			if err != nil {
				return err
			}
			u.checksum = fmt.Sprintf("%x", hasher.Sum(nil))
		}

		mcfile, err := u.fileStor.CreateFile(u.Filename, u.ProjectID, u.DirectoryID, u.OwnerID, mc.GetMimeType(u.Filename))
		if err != nil {
			// Need to do cleanup
			return err
		}

		if err := os.Rename(u.getChunkPath(), mcfile.ToUnderlyingFilePath(u.MCFSDir)); err != nil {
			// Need to do cleanup
			return err
		}

		// Remove state file for chunk
		_ = os.Remove(u.getStatePath())

		finfo, err := os.Stat(mcfile.ToUnderlyingFilePath(u.MCFSDir))
		if err != nil {
			return err
		}

		_, err = u.fileStor.DoneWritingToFile(mcfile, u.checksum, finfo.Size(), u.conversionStor)

		return err
	}

	return nil
}

// createFile creates the file with the content. If the corresponding directory does not exist,
// it is created. If the file already exists, its content is removed.
func createFile(path string, content []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		if os.IsNotExist(err) {
			// An upload ID containing slashes is mapped onto different directories on disk,
			// for example, `myproject/uploadA` should be put into a folder called `myproject`.
			// If we get an error indicating that a directory is missing, we try to create it.
			if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
				return fmt.Errorf("failed to create directory for %s: %s", path, err)
			}

			// Try creating the file again.
			file, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
			if err != nil {
				// If that still doesn't work, error out.
				return err
			}
		} else {
			return err
		}
	}

	if content != nil {
		if _, err := file.Write(content); err != nil {
			return err
		}
	}

	return file.Close()
}

func getStatePathByID(mcfsDir, ID string) string {
	return filepath.Join(mcfsDir, "__tus", fmt.Sprintf("%s.state", ID))
}

func getChunkPathByID(mcfsDir, ID string) string {
	return filepath.Join(mcfsDir, "__tus", ID)
}

// saveState saves the upload state in JSON format.
func (u *MCFileUpload) saveState() error {
	data, err := json.Marshal(u)
	if err != nil {
		return err
	}

	return createFile(u.getStatePath(), data)
}
