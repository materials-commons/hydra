package obj

import (
	"reflect"
)

func IsNil(what interface{}) bool {
	if what == nil {
		return true
	}

	kind := reflect.ValueOf(what).Kind()
	switch kind {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan:
		return reflect.ValueOf(what).IsNil()
	default:
		return false
	}
}
