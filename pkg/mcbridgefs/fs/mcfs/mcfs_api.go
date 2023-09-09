package mcfs

// MCFSApi is the file system interface into Materials Commons. It has little knowledge of
// FUSE. It understands the Materials Commons calls to make to achieve certain file system
// operations, and returns the results in a way that the node can pass back.
type MCFSApi struct {
}

func NewMCFSApi() *MCFSApi {
	return nil
}

func (fs *MCFSApi) Readdir() {

}

func (fs *MCFSApi) Getattr() {

}

func (fs *MCFSApi) Lookup() {

}

func (fs *MCFSApi) Mkdir() {

}

func (fs *MCFSApi) Create() {

}

func (fs *MCFSApi) Open() {

}

// Release Move out of the file handle?
func (fs *MCFSApi) Release() {

}
