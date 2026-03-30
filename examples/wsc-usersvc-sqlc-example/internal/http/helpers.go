package transport

import "strconv"

func int64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}
