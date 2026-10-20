//go:build android

package main

/*
#cgo LDFLAGS: -llog
#include <android/log.h>
#include <stdlib.h>

static void takt_go_log(const char *msg) {
	__android_log_write(ANDROID_LOG_INFO, "GoProbe", msg);
}
*/
import "C"

import "unsafe"

// logInfo writes to logcat (tag "GoProbe"). Go's own stderr is not forwarded
// to logcat in a plain c-shared library, so we call the NDK logger directly.
func logInfo(msg string) {
	cs := C.CString(msg)
	defer C.free(unsafe.Pointer(cs))
	C.takt_go_log(cs)
}
