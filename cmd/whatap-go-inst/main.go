// Command whatap-go-inst is the entry point for `go install`.
//
// The root main.go is the canonical build target (used by goreleaser and
// `go build .`); this thin wrapper exists so that
//
//	go install github.com/whatap/go-api-inst/cmd/whatap-go-inst@latest
//
// installs a binary named `whatap-go-inst` (matching the release tarball),
// as documented in docs/user-guide.md and README.md.
package main

import "github.com/whatap/go-api-inst/cmd"

func main() {
	cmd.Execute()
}
