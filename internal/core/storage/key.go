package storage

import (
	"os"
	"strings"
)

func ExtractKey(url string) string {
	base := os.Getenv("DO_SPACE_PUBLIC_BASE") + "/"
	return strings.TrimPrefix(url, base)
}
