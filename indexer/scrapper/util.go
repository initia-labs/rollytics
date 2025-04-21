package scrapper

import (
	"strings"
)

func reachedLatestHeight(errString string) bool {
	return strings.HasPrefix(errString, "current height") || strings.HasPrefix(errString, "could not find")
}
