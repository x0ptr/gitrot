package utils

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

func FormatAuthorName(name string, hideName bool) string {
	trimmed := strings.TrimSpace(name)
	if !hideName || trimmed == "" {
		return trimmed
	}
	sum := sha256.Sum256([]byte(trimmed))
	return fmt.Sprintf("usr-%x", sum)[:12]
}
