//go:build android

package main

/*
#include <stdint.h>
*/
import "C"

import (
	"fmt"

	"github.com/shntsk/taktchase/framework/graphics/batch"
)

// rendererFixture is created and used from SDL's render thread while its GLES
// context is current. The native bridge must not call these functions off-thread.
var rendererFixture *batch.AndroidBatchFixture

//export TaktRendererInit
func TaktRendererInit() (result C.int32_t) {
	defer func() {
		if recovered := recover(); recovered != nil {
			logInfo(fmt.Sprintf("TAKT_RENDER quadbatch_init=FAIL panic=%v", recovered))
			result = 0
		}
	}()
	if rendererFixture != nil {
		return 1
	}
	rendererFixture = batch.NewAndroidBatchFixture()
	logInfo("TAKT_RENDER quadbatch_init=PASS backend=GLES3 path=QuadBatch")
	return 1
}

//export TaktRendererDraw
func TaktRendererDraw() (result C.int32_t) {
	defer func() {
		if recovered := recover(); recovered != nil {
			logInfo(fmt.Sprintf("TAKT_RENDER quadbatch_draw=FAIL panic=%v", recovered))
			result = 0
		}
	}()
	if rendererFixture == nil {
		logInfo("TAKT_RENDER quadbatch_draw=FAIL reason=not_initialized")
		return 0
	}
	if !rendererFixture.Draw() {
		logInfo("TAKT_RENDER quadbatch_draw=FAIL reason=GLES_error")
		return 0
	}
	return 1
}

//export TaktRendererDestroy
func TaktRendererDestroy() (result C.int32_t) {
	defer func() {
		if recovered := recover(); recovered != nil {
			logInfo(fmt.Sprintf("TAKT_RENDER quadbatch_destroy=FAIL panic=%v", recovered))
			result = 0
		}
	}()
	if rendererFixture != nil {
		rendererFixture.Dispose()
		rendererFixture = nil
	}
	logInfo("TAKT_RENDER quadbatch_destroy=PASS")
	return 1
}
