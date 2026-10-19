package main

/*
#ifdef _WIN32
#include <windows.h>
// force switch to the high performance gpu in multi-gpu systems (mostly laptops)
__declspec(dllexport) DWORD NvOptimusEnablement = 0x00000001; // http://developer.download.nvidia.com/devzone/devcenter/gamegraphics/files/OptimusRenderingPolicies.pdf
__declspec(dllexport) DWORD AmdPowerXpressRequestHighPerformance = 0x00000001; // https://community.amd.com/thread/169965
#endif
*/
import "C"

import (
	"github.com/shntsk/taktchase/app"
	"github.com/shntsk/taktchase/framework/env"
	"github.com/shntsk/taktchase/launcher"
	"os"
)

func main() {
	env.Init("taktchase")

	if len(os.Args) == 1 {
		launcher.StartLauncher()
	} else {
		app.Run()
	}
}
