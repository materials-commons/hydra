package mql

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/feather-lang/feather"
)

// ToTclString converts a Go value to a TCL string representation.
// This code was copied over from feather.
func ToTclString(v any) string {
	if v == nil {
		return "{}"
	}

	switch val := v.(type) {
	case string:
		return quote(val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "1"
		}
		return "0"
	case []string:
		parts := make([]string, len(val))
		for i, s := range val {
			parts[i] = quote(s)
		}
		return strings.Join(parts, " ")
	case *feather.Obj:
		if val == nil {
			return "{}"
		}
		return quote(val.String())
	default:
		// Use reflection for other types
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array:
			parts := make([]string, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				parts[i] = ToTclString(rv.Index(i).Interface())
			}
			return strings.Join(parts, " ")
		case reflect.Map:
			var parts []string
			iter := rv.MapRange()
			for iter.Next() {
				parts = append(parts, ToTclString(iter.Key().Interface()))
				parts = append(parts, ToTclString(iter.Value().Interface()))
			}
			return strings.Join(parts, " ")
		default:
			return quote(fmt.Sprintf("%v", v))
		}
	}
}

// quote adds braces around a string if it contains special characters.
func quote(s string) string {
	if s == "" {
		return "{}"
	}
	needsQuote := false
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '\n' || c == '{' || c == '}' || c == '"' || c == '\\' || c == '$' || c == '[' || c == ']' {
			needsQuote = true
			break
		}
	}
	if needsQuote {
		return "{" + s + "}"
	}
	return s
}
