package mcdav

import (
	"strings"
)

func pathIsOnlyForProjectSlug(path string) bool {
	pieces := strings.Split(path, "/")
	return len(pieces) == 2
}
