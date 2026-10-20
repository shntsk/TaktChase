package main

/*
#include <stdint.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// Exported C ABI of the probe. Only C-compatible scalar types and plain
// pointers cross the boundary; no Go strings, slices, interfaces or callbacks.
// The hand-written header android/app/jni/src/taktgo_api.h must stay in sync
// with these signatures (scripts/verify-apk.sh checks the exported symbols).

func init() {
	// Runs when the dynamic loader maps libtaktgo.so (c-shared init).
	logInfo("Go runtime initialised inside libtaktgo.so: " + describe())
}

//export TaktProbeAdd
func TaktProbeAdd(a, b C.int32_t) C.int32_t {
	r := add(int32(a), int32(b))
	logInfo(fmt.Sprintf("TaktProbeAdd(%d, %d) = %d", int32(a), int32(b), r))
	return C.int32_t(r)
}

//export TaktProbeSumParallel
func TaktProbeSumParallel(n C.int32_t) C.int64_t {
	r := sumParallel(int32(n), 4)
	logInfo(fmt.Sprintf("TaktProbeSumParallel(%d) = %d (4 goroutines)", int32(n), r))
	return C.int64_t(r)
}

// TaktProbeDescribe writes a NUL-terminated description into the caller-owned
// buffer buf (capacity bytes) and returns the string length, or -1 on error.
//
//export TaktProbeDescribe
func TaktProbeDescribe(buf *C.char, capacity C.int32_t) C.int32_t {
	if buf == nil || capacity <= 0 {
		return -1
	}
	dst := unsafe.Slice((*byte)(unsafe.Pointer(buf)), int(capacity))
	return C.int32_t(copyToBuffer(dst, describe()))
}

// main is required by -buildmode=c-shared but is never called.
func main() {}
