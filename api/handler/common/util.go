package common

import (
	"fmt"
	"strings"
)

func GetNextKey(values ...any) []byte {
	if len(values) == 0 {
		return nil
	}

	var parts []string
	for _, v := range values {
		parts = append(parts, fmt.Sprintf("%v", v))
	}

	nextKey := strings.Join(parts, "|")
	return []byte(nextKey)
}
