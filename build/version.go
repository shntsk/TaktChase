package build

import (
	"runtime/debug"
)

var ProgramName = "TaktChase"

var CommitHash = "Unknown"

var VERSION = "dev"

var Stream = "Dev"

var TaktChaseExec = "taktchase-cli"

func init() {
	if bI, ok := debug.ReadBuildInfo(); ok {
		for _, k := range bI.Settings {
			if k.Key == "vcs.revision" {
				CommitHash = k.Value

				break
			}
		}
	}

	if VERSION == "dev" {
		VERSION += "-" + CommitHash[:min(7, len(CommitHash))]
	}
}
