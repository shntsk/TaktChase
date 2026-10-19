package main

import "C"

import (
	"github.com/shntsk/taktchase/app"
	"github.com/shntsk/taktchase/framework/env"
	"github.com/shntsk/taktchase/launcher"
	"os"
)

//export taktchaseMain
func taktchaseMain(isLauncher bool, args []string) {
	os.Args = args

	env.Init("taktchase")
	if isLauncher {
		launcher.StartLauncher()
	} else {
		app.Run()
	}
}
