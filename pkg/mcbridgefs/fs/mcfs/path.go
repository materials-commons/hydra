package mcfs

import (
	"path/filepath"
	"strings"
)

func ToProjectPath(p string) string {
	pathParts := strings.Split(p, "/")
	// pathParts[0] = ""
	// pathParts[1] = project-uuid
	// pathParts[2] = user-uuid
	// pathParts[...] = path to use for project path

	// The project root starts with a slash, so add a "/" into the list of
	// path parts we are going to join.
	pathPieces := append([]string{"/"}, pathParts[3:]...)
	return filepath.Join(pathPieces...)
}
