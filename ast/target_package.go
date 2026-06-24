package ast

import "strings"

// ExtractRulePackage extracts the package import path from a Rule.Target.
//
// §240 / §242. Examples:
//
//	"database/sql.Open"                        → "database/sql"
//	"github.com/gin-gonic/gin.Engine.Use"       → "github.com/gin-gonic/gin"
//	"github.com/redis/go-redis/v9.NewClient"    → "github.com/redis/go-redis/v9"
//	"net/http.Client{}"                         → "net/http"
//	"decl:github.com/foo/bar.Baz"               → "github.com/foo/bar"
//
// The Go convention — package names are lowercase, exported symbols start
// with an uppercase letter — lets us split at the first `.` inside the last
// `/`-segment. Struct-literal markers (`{`) and the `decl:` prefix are
// stripped first.
func ExtractRulePackage(target string) string {
	target = strings.TrimPrefix(target, "decl:")
	if idx := strings.Index(target, "{"); idx >= 0 {
		target = target[:idx]
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}

	slashIdx := strings.LastIndex(target, "/")
	prefix := ""
	lastSegment := target
	if slashIdx >= 0 {
		prefix = target[:slashIdx+1]
		lastSegment = target[slashIdx+1:]
	}

	dotIdx := strings.Index(lastSegment, ".")
	if dotIdx < 0 {
		// No "." — malformed or bare package name. Return as-is.
		return target
	}
	return prefix + lastSegment[:dotIdx]
}
