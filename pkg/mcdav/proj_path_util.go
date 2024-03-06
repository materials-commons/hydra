package mcdav

import (
	"strings"
)

func pathIsOnlyForProjectSlug(path string) bool {
	pieces := strings.Split(path, "/")
	if len(pieces) == 3 {
		return pieces[2] == ""
	}
	return false
}
