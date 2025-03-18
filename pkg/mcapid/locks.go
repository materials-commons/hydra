package mcapid

import (
	"github.com/materials-commons/hydra/pkg/lock"
)

var ProjectLocks = lock.NewIdLocker()
var FileTransferLocks = lock.NewIdLocker()
