package fsapi

import (
	"strconv"
)

func SafeParseInt64(s string) int64 {
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil {
		return 0
	}
	return n
}

func SafeParseBool(s string) bool {
	b, e := strconv.ParseBool(s)
	if e != nil {
		return false
	}
	return b
}
