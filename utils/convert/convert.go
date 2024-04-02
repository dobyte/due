package convert

import (
	"fmt"
	"strconv"
)

func ToString(n interface{}) string {
	switch v := n.(type) {
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	default:
		return fmt.Sprintf("%v", n)
	}
}
