package main

import (
	"fmt"
	"runtime"
	"sync"
)

// This file holds the plain-Go logic of the feasibility probe. It deliberately
// does not import "C" so it can be unit-tested with `go test` on any host.

const probeMagic = 0x7a74 // "zt": value mixed into Describe output, only to make it recognisable

// add returns a+b. It is the simplest possible round trip across the C ABI.
func add(a, b int32) int32 { return a + b }

// sumParallel sums 1..n using several goroutines, proving that the Go
// scheduler, channels and sync primitives work inside the c-shared library.
func sumParallel(n int32, workers int) int64 {
	if n <= 0 || workers <= 0 {
		return 0
	}
	var wg sync.WaitGroup
	partial := make([]int64, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			var s int64
			for i := int64(w) + 1; i <= int64(n); i += int64(workers) {
				s += i
			}
			partial[w] = s
		}(w)
	}
	wg.Wait()
	var total int64
	for _, s := range partial {
		total += s
	}
	return total
}

// describe reports the Go runtime the library was built with.
func describe() string {
	return fmt.Sprintf("go=%s os=%s arch=%s cpus=%d magic=%#x",
		runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), probeMagic)
}

// copyToBuffer copies s into dst as a NUL-terminated string, truncating if
// needed. It returns the number of bytes written excluding the NUL, or -1 if
// dst cannot even hold the terminator.
func copyToBuffer(dst []byte, s string) int32 {
	if len(dst) == 0 {
		return -1
	}
	n := copy(dst[:len(dst)-1], s)
	dst[n] = 0
	return int32(n)
}
