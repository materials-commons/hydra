package fs

import (
	"strings"
)

// pathIsOnlyForProjectSlug determines whether the path points to a project-slug
// path, or to an entry under the project-slug. The check is not very obvious.
// A potential project-slug path has length 3, but there are two cases for a
// path with length 3:
//  1. /<project-slug>/ -> This is a project-slug path. When split it returns ["", <project-slug>, ""]
//  2. /<project-slug/file-or-dir -> This is NOT a project-slug path. When split it returns ["", <project-slug>, "file-or-dir"]
//
// When length is NOT 3 we return false as there is no chance it's a project-slug path. Though it never seems to happen
// we also account for /<project-slug (no ending "/"). Note in this case that len(strings.Split("/", "/") == 2
// and len(strings.Split("/slug", "/")) == 2. To account for this we explicitly check for path == "/".
func pathIsOnlyForProjectSlug(path string) bool {
	if path == "/" {
		return false
	}

	pieces := strings.Split(path, "/")
	if len(pieces) == 2 {
		return true
	}

	if len(pieces) == 3 {
		return pieces[2] == ""
	}
	return false
}
