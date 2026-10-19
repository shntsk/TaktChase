package utils

import (
	"github.com/shntsk/taktchase/framework/goroutines"
	"github.com/shntsk/taktchase/framework/platform/gcontext"

	"os"
	"os/exec"
)

func QuickRestart() {
	taktchasePath := os.Args[0]

	arguments := make([]string, 0)

	noDbCheck := false
	quickStart := false

	for _, arg := range os.Args[1:] {
		if arg == "-nodbcheck" {
			noDbCheck = true
		}

		if arg == "-quickstart" {
			quickStart = true
		}

		arguments = append(arguments, arg)
	}

	if !noDbCheck {
		arguments = append(arguments, "-nodbcheck")
	}

	if !quickStart {
		arguments = append(arguments, "-quickstart")
	}

	cmd := exec.Command(taktchasePath, arguments...)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Start()

	goroutines.CallNonBlockMain(func() {
		gcontext.SetShouldClose(true)
	})
}
