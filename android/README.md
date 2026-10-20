# TaktChase Android — Stage 3 renderer checkpoint

> **Status: partial renderer work, not a playable Android port.** Stage 2's SDL3/GLES/Go feasibility foundation is retained. Stage 3 adds a runtime compile/link and interface check for the game's real sprite shaders. The Android app still draws the original diagnostic triangle; it does **not** draw a real TaktChase hit object or gameplay scene yet.

## What Stage 3 changes

- CMake derives an Android/GLES shader header directly from `assets/shaders/sprite.vsh` and `assets/shaders/sprite.fsh`. It converts only the first-line `#version 330` directive to `#version 300 es` and adds the fragment shader's required default float precision. The desktop shader files remain untouched and are the source of truth.
- After SDL creates an OpenGL ES 3.0 context, native code compiles and links both converted shaders and checks that the 11 sprite vertex attributes and the `proj` / `tex` uniforms survive linking. A successful runtime test emits `TAKT_RENDER sprite_shader_compile=PASS`.
- The x86_64 emulator smoke test now requires this marker in addition to the existing Go, GLES, diagnostic-pixel and lifecycle markers.
- The emulator smoke job runs automatically on pushes to `android/port-stage3` and fails the workflow if it fails. It can also be enabled manually on `workflow_dispatch` by setting `run_emulator=true`.
- The ARM64 APK remains the primary build artifact. The x86_64 emulator APK is a separate diagnostic build and does not prove the ARM64 APK works on a physical phone.

This is intentionally a shader compatibility checkpoint, **not** a rendering claim: compilation does not prove the game's batch pipeline, vertex data, texture uploads, blending or a hit object can render through GLES.

## Renderer audit: the bridge boundary and remaining blockers

The native Android Gradle build still compiles only `android/app/jni/src/taktchase_main.c` and `go_bridge.c`, and links the separate probe module under `android/go/`. It does not yet link the root Go game module. The root module now contains both renderer backends behind the `QuadBatch` interface, but the Android backend is not wired into this APK or runtime-tested. The triangle remains only a diagnostic scene.

The first source-level refactor creates a renderer seam in the existing game code:

- `framework/graphics/batch/batch.go` now owns the same public `QuadBatch` API, transform calculations, sprite ordering and packed instance data. It sends GPU work through the internal `spriteBatchRenderer` interface.
- `framework/graphics/batch/renderer_desktop.go` contains the existing desktop OpenGL resource setup and draw calls. This preserves the current desktop implementation behind the interface and is excluded from Android builds with a build constraint.
- `framework/graphics/batch/renderer_android.go` now implements the same interface using GLES 3 calls, a CPU-side instance buffer and offset-zero uploads followed by `glDrawElementsInstanced`. This avoids relying on desktop `baseInstance`. It has not yet been compiled as part of the Android game build or run on an emulator/device.

The remaining platform-specific work includes:

- `framework/graphics/buffer/vao.go` uses desktop DSA VAO calls such as `CreateVertexArrays`, `EnableVertexArrayAttrib`, `VertexArrayAttribBinding`, `VertexArrayAttribFormat`, `VertexArrayVertexBuffer` and `VertexArrayBindingDivisor`.
- `framework/graphics/buffer/vbo.go` and `ibo.go` use `CreateBuffers`, `NamedBufferData` and `NamedBufferSubData`; indexed instanced drawing calls `DrawElementsInstancedBaseInstance`.
- `framework/graphics/texture/texture.go` uses desktop DSA texture calls including `CreateTextures`, `TextureStorage3D`, `TextureParameteri` and `TextureSubImage3D`.
- `framework/graphics/shader/rshader.go` depends on the desktop `github.com/go-gl/gl/v3.3-core/gl` binding and direct program-uniform APIs such as `ProgramUniform1fv`.
- The Go batch API still imports the existing texture package, whose resource implementation also needs a GLES backend before the root gameplay module can be built for Android.

The new GLES batch backend uses ordinary GLES buffer allocation/update, vertex attribute setup, bound-program uniforms and offset-zero uploads followed by `glDrawElementsInstanced`; it does not depend on desktop base-instance support. The remaining resource blocker is texture creation/upload: `framework/graphics/texture/texture.go` still calls desktop DSA functions, and the root gameplay module has not yet been connected to the Gradle/SDL/JNI build. Those must be ported before the backend can draw actual game content.

The generated sprite shader test verifies only shader syntax and interface. It doesn't prove the quad batch, its textures, or hit objects can yet render on Android.

## Current evidence status

| Check | State |
|---|---|
| Stage 2 source and Android project inspected | Done |
| Desktop sprite shaders adapted from their existing source files | Implemented in CMake generation; runtime result awaits CI |
| GLES runtime shader compile/link and shader-interface marker | Implemented; runtime result awaits CI |
| Existing Go self-test | Retained in the SDL thread and JNI bridge; CI/device result must be read from its run |
| Existing diagnostic triangle read-back | Retained; it is still only a prototype scene |
| Emulator startup and background/foreground test | Scheduled automatically for pushes to this branch; not claimed passed until the Actions run reports success |
| ARM64 APK built and statically verified in Stage 3 | Must be confirmed by this branch's Actions run |
| Runtime tested on a physical ARM64 phone | **Not verified** |
| Real TaktChase hit object/playfield rendered on GLES | **Not implemented yet** |

Do not report emulator checks as passed if the job was skipped or failed. An x86_64 emulator is not an ARM64 physical device.

## Layout and architecture carried forward from Stage 2

```
android/
  go/                isolated Go probe module (not the game module)
  gomobile-probe/    informational gomobile-bind experiment
  app/               Gradle app, SDL3 native entry point, JNI/Go bridge
    jni/src/         SDL_main, JNI bridge, C ABI and CMake
  scripts/           pinned SDL fetch, Go c-shared build, APK checks and emulator smoke
  SDL_VERSION        pinned SDL source commit
.github/workflows/android-debug.yml
                     ARM64 build and verification plus an x86_64 emulator test
```

The native bridge crosses into Go only with scalar C types and plain pointers. It intentionally does not expose the desktop `taktchaseMain(bool, []string)` Go signature to JNI. The existing Go probe, SDL startup, GLES 3.0 context creation, pixel read-back and Activity lifecycle checks remain in place.

## Pinned toolchain

| Component | Version |
|---|---|
| SDL3 | `release-3.4.18`, commit `829a65d769d935c4852f8159e964312c0957260a` |
| Go | `1.27.2`, `GOTOOLCHAIN=local` |
| Android Gradle Plugin / Gradle | `8.7.3` / `8.12` |
| NDK / CMake / build-tools / platform | `28.2.13676358` / `3.22.1` / `35.0.0` / `android-35` |
| JDK | Temurin 17 |
| `minSdk` / `targetSdk` | 24 / 35 |
| Primary APK ABI | `arm64-v8a` |
| Emulator smoke ABI | `x86_64` |

## Build locally

Requirements: JDK 17, Go 1.27.2, Android SDK components above, `ANDROID_HOME`, and `ANDROID_NDK_HOME`.

```sh
cd /path/to/TaktChase
(
  cd android/go
  GOTOOLCHAIN=local go vet ./...
  GOTOOLCHAIN=local go test ./...
)
android/scripts/fetch-sdl.sh
android/scripts/build-go.sh arm64-v8a
(cd android && gradle --no-daemon --stacktrace assembleDebug -PtaktAbi=arm64-v8a)
android/scripts/verify-apk.sh android/app/build/outputs/apk/debug/app-debug.apk arm64-v8a
```

CMake uses `android/scripts/generate-gles-sprite-header.py` during the native build to create a generated header under the CMake build directory. Do not hand-edit that generated file. The shader adapter deliberately fails if either desktop shader's expected version directive changes, so a source change forces an explicit review.

To run the emulator smoke test, build a separate x86_64 APK with `android/scripts/build-go.sh x86_64`, assemble with `-PtaktAbi=x86_64`, boot an x86_64 Android emulator, then run:

```sh
android/scripts/emulator-smoke.sh android/app/build/outputs/apk/debug/app-debug.apk logcat-smoke.txt
```

The script checks startup, Go calls, GLES version, diagnostic pixels, sprite shader compile/link/interface, heartbeat, and a post-resume pixel read-back that proves a frame was drawn after foreground. Its log is uploaded as a workflow artifact on success or failure.

## CI and artifact policy

`.github/workflows/android-debug.yml` is independent of the desktop release workflow.

1. **ARM64 build:** pinned JDK/Go/SDK/NDK/Gradle; Go `vet` and tests; fetch pinned SDL3; build Go `c-shared`; run `assembleDebug`; verify APK ABI, native libraries, symbols and ELF metadata; then upload the APK artifact.
2. **Emulator smoke:** automatically runs for pushes to `android/port-stage3`. It creates a separate x86_64 diagnostic APK and runs the smoke script on a hosted emulator. It is blocking for this branch; a failure must be fixed rather than marked non-blocking. Manual runs can opt in with `run_emulator=true`.
3. **gomobile experiment:** remains informational and cannot substitute for the selected C/JNI bridge or for the APK/emulator tests.

Build logs are uploaded on build failure; emulator logcat/native diagnostics are uploaded regardless of the smoke result. The APK is uploaded only after its build and static verification steps pass.

## Out of scope

Full renderer migration, actual hit-object drawing, ImGui launcher/song selection, beatmap import/storage, touch gameplay, production audio, BASS binaries and FFmpeg/storyboard integration are still outside the completed scope. Do not redistribute BASS or other third-party binaries without verifying their Android availability and licence. Preserve GPLv3 and third-party notices.

The current branch contains a shader portability checkpoint, not a complete or playable Android game. A real gameplay rendering path and physical-device validation remain future work.
