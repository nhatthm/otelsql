package suite

import (
	"regexp"
	"strings"
)

var (
	alphaNum = regexp.MustCompile("[^a-zA-Z0-9]+")
)

func envVarName(parts ...string) string {
	return strings.ToUpper(alphaNum.ReplaceAllString(strings.Join(parts, "_"), "_"))
}
