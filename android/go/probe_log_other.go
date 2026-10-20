//go:build !android

package main

import (
	"fmt"
	"os"
)

// logInfo is the host fallback used for local tests and non-Android builds.
func logInfo(msg string) { fmt.Fprintln(os.Stderr, "GoProbe: "+msg) }
