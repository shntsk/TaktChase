# TaktChase Android — stage 2: native feasibility prototype

> **This is not the ported game.** It is a minimal prototype that answers two
> questions from `ANDROID_AUDIT.md`: *can we build and start an Android app with
> SDL3 + OpenGL ES?* and *can Go be integrated with SDL3 in a sustainable way?*
> It does not start the ImGui launcher, gameplay, map import, or audio.

## Status of the evidence (read this first)

| Claim | State |
|---|---|
| Go probe passes `go vet`/`go test`; c-shared build exports the C ABI; a C harness loads it and gets `42` | **Verified on a Linux x86_64 host** (Go 1.22.2, not Android) |
| Native C/JNI sources compile (syntax/type check) against the real SDL 3.4.18 and GLES3 headers | **Verified on host** (`gcc -fsyntax-only`, Android log header stubbed) |
| Java sources type-check | **Verified against minimal stubs only** (no `android.jar`) |
| `scripts/verify-apk.sh` accepts a correct package and rejects stray files | **Verified on host** with a synthetic x86_64 package |
| Workflow YAML and shell scripts are well-formed | **Verified** (actionlint + shellcheck) |
| Cross-compiling Go with the NDK (`GOOS=android`, c-shared) | **Not yet run** — needs the NDK; first real run is the CI job |
| SDL3 builds for Android, Gradle `assembleDebug` succeeds, APK is produced | **Not yet run** — needs Android SDK/NDK/Gradle (CI job) |
| App opens on Android, GLES 3.x context, scene draws, Go is called, logcat output | **Not yet run on any Android device or emulator** |
| `gomobile bind` works for this probe | **Not yet run** (informational CI job) |

The authoring environment had no Android SDK/NDK and could not reach `go.dev`,
`dl.google.com`, `maven.google.com` or `services.gradle.org`, so nothing Android-specific
could be executed there. Treat every "Not yet run" row as unproven until the
`Android debug (feasibility prototype)` workflow has run and the APK has been
started on a device.

## Layout

```
android/
  go/                Go module (isolated from the root module): the probe library
  gomobile-probe/    Go module for experiment B (gomobile bind), run only in CI
  app/               Gradle module: Activity, JNI/SDL_main sources, CMake
    jni/src/         taktchase_main.c (SDL3+GLES), go_bridge.c (JNI + Go calls), taktgo_api.h
    jni/SDL/         SDL3 checkout, fetched at a pinned commit (git-ignored)
    src/main/jniLibs libtaktgo.so built by scripts/build-go.sh (git-ignored)
  scripts/           fetch-sdl.sh, build-go.sh, verify-apk.sh, gomobile-probe.sh, emulator-smoke.sh
  SDL_VERSION        pinned SDL tag + commit
.github/workflows/android-debug.yml   independent CI workflow
```

Nothing under the existing desktop code or `release.yml` was modified. The Go
probe lives in a nested module (`android/go/go.mod`), so `go build ./...` in the
repository root does not see it.

## Architecture chosen: Go as a C shared library

```
Java TaktActivity (extends SDLActivity)
   ├─ loads libSDL3.so, libtaktgo.so (Go), libmain.so
   ├─ JNI  GoBridge.nativeAdd/nativeDescribe ──┐
   └─ SDL thread: SDL_main (taktchase_main.c) ─┴─► go_bridge.c ─► libtaktgo.so (C ABI)
```

Only scalar C types and plain pointers cross the boundary
(`int32_t TaktProbeAdd(int32_t,int32_t)`, `int64_t TaktProbeSumParallel(int32_t)`,
`int32_t TaktProbeDescribe(char*,int32_t)`). The desktop
`taktchaseMain(isLauncher bool, args []string)` is intentionally **not** exposed.

### Options investigated

**A. CGO `-buildmode=c-shared` + C/JNI bridge — chosen.**
Build: `GOOS=android GOARCH=arm64 CGO_ENABLED=1 CC=<ndk>/aarch64-linux-android24-clang go build -buildmode=c-shared`
(see `scripts/build-go.sh`). Reasons: SDL3's Android model runs `SDL_main` in
C on its own thread, so C calling into Go is the natural direction; the C ABI is
small and explicit; Go can later own the game loop and be driven from C
(`init`, `frame`, `touch`, `pause`) without going through Java.
Details handled in the scripts: an explicit `-soname` (otherwise dependents can
record the build path as `DT_NEEDED`), 16 KB `max-page-size`, API 24 for both
Go and the app (`minSdk 24`), and a CMake imported target that fails the build
with a clear message if `libtaktgo.so` is missing.
Host evidence only so far (see table above).

**B. `gomobile bind` / gobind — not chosen, to be tested in CI.**
`gomobile bind` produces an AAR with generated Java wrappers around a Go
package. Expected drawbacks for this project (hypotheses to confirm from the CI
log, not measured facts): the API is Java-first, so C code on SDL's thread would
have to reach Go through JNI → Java → Go, and gobind restricts parameter types
(no arbitrary Go structs/callbacks), which fits poorly with a game loop driven
from native code; it also adds a second, separate native library to keep in step
with SDL's. The `gomobile-experiment` job runs `gomobile bind -target=android/arm64`
on a small gobind-compatible package (`gomobile-probe/`), pinned to
`golang/mobile@19777044486779222400e9ca16c4306f9c6dbfa9`, and writes the real
result to the job summary. Its outcome does not gate the APK.

If A fails in CI, the error is the first thing to record here; B is then the
fallback to test before concluding the approach is unworkable.

## Pinned versions

| Component | Version |
|---|---|
| SDL3 | `release-3.4.18` (`829a65d769d935c4852f8159e964312c0957260a`) |
| Go | `1.27.2` (as in `go.mod` / `release.yml`), `GOTOOLCHAIN=local` |
| Android Gradle Plugin / Gradle | `8.7.3` / `8.12` (from SDL's reference template) |
| NDK / CMake / build-tools / platform | `28.2.13676358` / `3.22.1` / `35.0.0` / `android-35` |
| JDK | Temurin 17 |
| `minSdk` / `targetSdk` / ABI | 24 / 35 / `arm64-v8a` |
| GitHub Actions | pinned to full commit SHAs in the workflow |

The copied Gradle wrapper JAR is not in Gradle's published checksum list. CI
recognizes only the recorded legacy SHA-256, restores the official Gradle 8.12
wrapper JAR from a pinned Gradle source commit, verifies its published SHA-256,
and then runs `gradle/actions/wrapper-validation`. The wrapper properties also
pin the Gradle 8.12 binary distribution using its official SHA-256 checksum.

## Building locally

Requirements: JDK 17, Go 1.27.2, Android SDK with the components above, `ANDROID_HOME`, `ANDROID_NDK_HOME`.

```sh
android/scripts/fetch-sdl.sh                       # SDL at the pinned commit
android/scripts/build-go.sh arm64-v8a              # libtaktgo.so -> app/src/main/jniLibs
(cd android && ./gradlew assembleDebug)            # APK: android/app/build/outputs/apk/debug/app-debug.apk
android/scripts/verify-apk.sh android/app/build/outputs/apk/debug/app-debug.apk
adb install -r android/app/build/outputs/apk/debug/app-debug.apk
adb logcat -s TaktChase GoProbe
```

## What a successful run looks like (logcat)

Tags `TaktChase` (C/Java) and `GoProbe` (Go). Expected markers:

```
GoProbe:   Go runtime initialised inside libtaktgo.so: go=go1.27.2 os=android arch=arm64 ...
TaktChase: TAKT_PROBE sdl_init=ok video_driver=...
TaktChase: TAKT_PROBE go_selftest=PASS            # C -> Go from the SDL_main thread
TaktChase: TAKT_PROBE java_jni_selftest=PASS      # Java -> JNI -> Go
TaktChase: TAKT_PROBE gl_version_string=OpenGL ES 3.x ...
TaktChase: TAKT_PROBE gles_major_minor=3.x
TaktChase: TAKT_PROBE scene_check=PASS            # read-back: corner = clear colour, centre = triangle
TaktChase: TAKT_PROBE heartbeat frames=300
```

Lifecycle handling: drawing stops on `WILL/DID_ENTER_BACKGROUND` and resumes on
`DID_ENTER_FOREGROUND`; Activity `onCreate/onPause/onResume/onDestroy` are
logged. Whether the EGL context/surface survive a background/foreground cycle
on real drivers is exactly what still needs a device test.

## CI

`.github/workflows/android-debug.yml` (separate from `release.yml`):

1. `build`: Java 17, Go 1.27.2 (asserted), Android SDK/NDK/CMake → Go vet+tests →
   fetch SDL → Go c-shared for arm64 → `assembleDebug` → `verify-apk.sh` →
   upload artifact `taktchase-probe-debug-arm64-v8a`. Any failing step stops the
   job; no APK is uploaded unless verification passed.
2. `gomobile-experiment`: experiment B, non-blocking, result in the job summary.
3. `emulator-smoke` (manual, `run_emulator=true`): builds a **separate x86_64**
   diagnostic APK and runs `emulator-smoke.sh` on a hosted x86_64 emulator
   (start-up markers, Go calls, GLES check, background/foreground). It is
   non-blocking and untested. The arm64 deliverable itself is **not** executed
   in CI; it needs a real device (or an arm64 emulator host).

## Prepared for later stages (nothing done yet)

- **Audio:** BASS / BASS FX / BASS Mix are not integrated and no binaries were
  added. Needed later: Android arm64 builds of the three libraries and a review
  of the licence — `CREDITS.md` states BASS is commercial and only free for
  non-commercial use. Availability of Android builds and exact terms must be
  verified at the vendor before redistribution; an alternative audio backend
  (e.g. via SDL3 audio) is also an option to evaluate.
- **libyuv / FFmpeg / ImGui launcher:** untouched; FFmpeg-as-subprocess has no
  Android equivalent, video/storyboard stays out of scope.
- **Renderer:** untouched (desktop GL 3.3 core, ~26 `#version 330` shaders,
  DSA/base-instance usage; see `ANDROID_AUDIT.md` §3.1). The probe's GLES scene
  only proves context creation, an ES 3.00 shader and one draw call.
- **Assets/licences:** Redd Glass HD skin and the danser logo have usage
  restrictions (`CREDITS.md`); none are in the APK. The project is GPLv3; any
  distributed APK needs source availability and notices.
