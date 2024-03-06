package mcdav

import (
	"fmt"
	"strings"
)

func pathIsOnlyForProjectSlug(path string) bool {
	fmt.Println("pathIsOnlyForProjectSlug:", path)
	pieces := strings.Split(path, "/")
	return len(pieces) == 2
}
