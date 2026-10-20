// Package taktprobe is the gomobile/gobind variant of the stage-2 probe. It
// only uses types gobind supports (int32, string); it is not the C ABI used
// by the main prototype.
package taktprobe

import (
	"fmt"
	"runtime"
)

// Add returns a+b.
func Add(a, b int32) int32 { return a + b }

// Describe reports the Go runtime the AAR was built with.
func Describe() string {
	return fmt.Sprintf("go=%s os=%s arch=%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
