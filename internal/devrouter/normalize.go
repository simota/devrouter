package devrouter

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"strings"
)

func NormalizeName(input string) string {
	if input == "" {
		return "app"
	}
	lower := strings.ToLower(input)
	var b strings.Builder
	b.Grow(len(lower))
	prevDash := false
	for i := 0; i < len(lower); i++ {
		ch := lower[i]
		isAlpha := ch >= 'a' && ch <= 'z'
		isNum := ch >= '0' && ch <= '9'
		if isAlpha || isNum {
			b.WriteByte(ch)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "app"
	}
	return out
}

func NormalizeRelPath(input string) string {
	if input == "" {
		return ""
	}
	clean := filepath.Clean(input)
	clean = filepath.ToSlash(clean)
	clean = strings.TrimPrefix(clean, "./")
	return clean
}

func StackID(stackName, repoPath string) string {
	name := NormalizeName(stackName)
	sum := sha1.Sum([]byte(repoPath))
	return fmt.Sprintf("%s-%x", name, sum[:4])
}

func ComposeServiceName(stackName, serviceName string) string {
	return fmt.Sprintf("%s-%s", NormalizeName(stackName), NormalizeName(serviceName))
}
