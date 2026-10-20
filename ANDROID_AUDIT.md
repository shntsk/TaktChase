# TaktChase Android Port Audit

**Audit date:** 2026-10-19  
**Scope:** Static review of the supplied TaktChase ZIP and its source structure  
**Status:** Audit only. No source files were changed and no builds or runtime tests were performed during this audit.

## Executive summary

TaktChase is a Go 1.27 / CGO rhythm-game project derived from danser-go and distributed under GPLv3. The codebase already contains an osu!standard beatmap parser, timing/object logic, a SQLite beatmap database, map import/unpacking, and a desktop `-play` path.

The most significant risk in an Android port is the rendering layer. The current renderer targets desktop OpenGL 3.3 core, uses many direct-state-access APIs and requires extensions that are not part of OpenGL ES. A successful Go/Android build alone would not demonstrate that the existing game can render or play on a phone.

The recommended approach is to prove the native Android integration in a small independent prototype first, then port the renderer and only afterwards add the Android map library and touch gameplay. This report distinguishes code observations from assumptions that still require compilation or device testing.

## 1. Current architecture

### Program entry points

- `main.go` initializes the environment with `env.Init`. Without command-line arguments it starts `launcher.StartLauncher()`; with arguments it starts `app.Run()`.
- `mainshared.go` exposes `taktchaseMain(isLauncher bool, args []string)` for a shared-library build. These parameters include Go types and should not be treated as a directly callable JNI/C ABI.
- The Linux build script produces a C-shared library and uses a small native C stub in `cmain/main_taktchase.c`.

### Launcher and gameplay startup

- The desktop launcher uses ImGui and the existing SDL/OpenGL layer.
- The launcher starts gameplay as a child process and passes command-line flags to it (`launcher/launcher.go`, around the process-launching code).
- `app/app.go` parses flags and settings, resolves the selected beatmap using flags such as `-id`, `-md5`, or artist/title criteria, initializes SDL and OpenGL, initializes BASS, loads beatmap objects and creates the gameplay player.
- In `-play` mode, gameplay input is handled by `dance.PlayerController`.
- The player clock is driven by the current BASS music position in a goroutine; input is read through polling in the controller.

### Beatmap import and selection

- `app/beatmap/parser.go` parses the standard `.osu` sections and their metadata, difficulty, timing points and hit objects.
- `app/beatmap/beatmap.go` owns parsed beatmap state and object/timing updates.
- `app/database/manager.go` scans the configured Songs directory, unpacks `.osz` archives, imports `.osu` metadata into SQLite and loads selectable beatmaps.
- The metadata IDs are read from local `.osu` files. A valid online ID is not inherently required for parsing a local map.
- `launcher/songselect.go` implements the existing desktop music/difficulty selector. Its UI cannot be reused as-is in a native Android menu, but the underlying map data and parser should be reusable.

## 2. Reusable components

The following areas appear to be useful foundations, subject to Android compilation and runtime verification:

- Beatmap parsing, timing-point processing and hit-object/ruleset logic.
- Beatmap metadata and difficulty representation.
- SQLite database/import logic, provided `go-sqlite3` compiles and runs with the Android NDK configuration.
- Existing `.osz` import and archive extraction logic, after fixing error handling and adapting paths/storage.
- The selected-map command-line path, which may be adapted to launch a game session by a local identifier such as MD5 rather than relying on an online ID.

The Android UI should be implemented separately from the desktop ImGui launcher. Reuse the core game logic where practical rather than coupling the Android UI to desktop launcher internals.

## 3. Main Android blockers

### 3.1 Desktop OpenGL vs. OpenGL ES — critical

The existing context initializes OpenGL 3.3 core. The extension check in `framework/platform/gcontext/gl.go` requires:

- `GL_ARB_clear_texture`
- `GL_ARB_direct_state_access`
- `GL_ARB_texture_storage`
- `GL_ARB_vertex_attrib_binding`
- `GL_ARB_buffer_storage`

These are desktop OpenGL extension requirements and cannot be assumed to exist in OpenGL ES. The renderer uses direct-state-access functions such as `CreateBuffers`, `CreateTextures`, `TextureStorage3D`, `NamedFramebuffer*` and `VertexArray*`.

Batch drawing also uses `DrawArraysInstancedBaseInstance` and related base-instance functionality in the buffer/VAO and batch code. GLES 3.0 does not provide the same desktop API as a drop-in replacement. Other incompatibilities identified in the audit include:

- `CopyImageSubData` in multilayer texture code.
- `ClearTexImage` in texture code.
- BGR/BGRA format assumptions in texture format handling.
- Approximately 26 shaders using `#version 330`.

The persistent-buffer path is optional and appears disabled by default, but other direct-state-access usage is widespread.

The current `go-gl/gl/v3.3-core/gl` binding may also use desktop GL/GLX build assumptions on Android. The Android build must not assume that setting `GOOS=android` and reusing the existing loader will work. GL function loading would need to use SDL's proc-address mechanism or another Android-compatible GLES binding.

**Recommended action:** first prove SDL3 + GLES rendering in an Android prototype. Afterwards, port the renderer systematically and test shaders on Android. Do not claim the renderer is ported merely because Go code compiles.

### 3.2 SDL3 integration — high risk, not yet runtime-verified

The current SDL layer loads the library at runtime through purego and constructs the path using `env.LibDir()`. The current window path requests OpenGL core 3.3. It also quits and reinitializes the video subsystem while creating a window, and starts text input.

For Android, verify the SDL3 Android Activity/lifecycle integration, surface recreation, GLES context attributes, keyboard behavior and the way the SDL native library is packaged. The existing Linux `libSDL3.so` is not an Android binary.

The current Go SDL wrapper lists Windows/Linux support in its README; actual Android support for the project's pinned version is unverified. Build a minimal experiment before relying on that wrapper.

Reference: [SDL3 Android README](https://wiki.libsdl.org/SDL3/README-android).

### 3.3 Go-to-Android bridge — critical

The exported function in `mainshared.go` accepts Go types (`bool` and `[]string`). These are not a JNI API signature that Kotlin/Java can invoke directly.

Use a supported integration strategy and prove it with a minimal callable Go function. Options to investigate include a CGO shared library with a C-compatible exported interface and JNI shim, or gomobile/gobind if the code and dependency graph permit it. Do not assume those approaches are interchangeable.

The bridge prototype must produce an observable result, such as a log message or returned value, from native Android code.

### 3.4 Current process model and global state

The desktop launcher runs gameplay as a child process. The game also contains global settings, player and audio state; `app/app.go` configures global command-line flags, and error paths use `os.Exit`.

This architecture makes restarting multiple game sessions inside a single long-lived Go runtime risky. A separate game process may be a reasonable initial Android design, but it must be proved with an actual Android process/Activity architecture rather than assumed.

### 3.5 File-system paths and assets — critical

`framework/env/paths.go` configures data, config and library directories relative to the executable in non-package mode and attempts to create a `settings` directory there. An Android APK's installed application directory is not a writable place for user data.

The port needs Android-private writable locations for settings, database, imported maps and cache. Assets currently loaded from the asset pack or library directory (the audit estimated around 12 MB) must be packaged and located in a way that works on Android.

Use Android's Storage Access Framework/document picker for importing files and copy selected `.osz` archives into app-private storage. Do not request broad storage access simply to import a map.

### 3.6 Touch input — high complexity

The current SDL event handling does not process `FINGER_*` events. The gameplay ruleset uses one cursor with left/right button state, and hit-object checks compare a cursor position against circles and sliders.

A mobile input layer needs to convert touch coordinates to playfield coordinates, handle short press events reliably (polling alone can miss a brief press), model button press/release state, and decide how multiple fingers interact with circles, sliders and spinners. Mouse-event emulation could be an early experiment, but it should not be assumed to be adequate for gameplay.

### 3.7 Lifecycle, pause and audio synchronization

The audit found no complete pause/resume path for Android lifecycle changes. The BASS track has pause behavior used in a failure path, but that is not a full application lifecycle implementation. The player's timing goroutine may continue while the app is backgrounded, and context-loss behavior has not been tested.

Add explicit pause/resume and audio/render lifecycle handling, including surface recreation and preventing gameplay time from running unintentionally in the background.

### 3.8 Native audio, video and third-party assets

- **BASS / BASS FX / BASS Mix:** the current CGO code links these libraries. Android ARM64-compatible builds and exact redistribution terms must be obtained and reviewed. The supplied project credits describe commercial licensing terms; do not bundle or publicly redistribute binaries until the applicable licence is confirmed.
- **libyuv:** used through native code in the video/storyboard path. It may need to be compiled for Android with the NDK or the dependent feature must be isolated.
- **FFmpeg:** some code invokes FFmpeg as an external executable, which is not a portable Android design. Video/storyboard functionality should remain disabled in the first mobile prototype unless deliberately ported.
- **Skin and logo assets:** the existing credits mention special usage restrictions for the Redd Glass HD skin and the inherited danser logo. Check these restrictions before redistributing a mobile package.

The project is identified as GPLv3 in the supplied audit context. Preserve the source licence and third-party notices and check obligations before publishing APKs.

### 3.9 Archive import error handling

The existing importer unpacks `.osz` files and indexes the contained `.osu` files. The audit noted that errors returned from the unzip path appear to be ignored and filename collisions need better handling. Even if extraction includes ZipSlip protection, Android import should report invalid/corrupt archives clearly, handle partial extraction safely and avoid deleting the original archive before a successful import.

### 3.10 Potential beatmap update bug

The audit flagged a possible slicing issue in `app/beatmap/beatmap.go`: the removal of processed objects appears to use `processed[toRemove:]` where `toRemove2` may be intended. This has not been confirmed against upstream or with a regression test. Verify the logic and add a focused test before changing it.

## 4. Likely files and areas involved

The following are candidate areas for later changes, not a claim that every file must be modified:

- **Renderer and shaders:** `framework/graphics/{buffer,texture,shader,batch,blend,viewport,effects}`, `app/graphics/sliderrenderer/*`, `app/utils/screenshot.go`, and `assets/shaders/*`.
- **Window/context and events:** `framework/platform/gcontext/gl.go`, `gcontext/sdl.go`, `gcontext/sdlevents.go`.
- **Paths and assets:** `framework/env/paths.go`, `framework/assets/manager.go`.
- **Input and gameplay lifecycle:** `app/dance/pcontroller.go`, `app/states/player.go`, `app/app.go`.
- **Beatmap storage/import:** `app/database/manager.go` and archive utility code.
- **New Android pieces:** Android Gradle project, Activity/SDL integration, C/JNI bridge (if selected), Android-specific Go entry point and a dedicated GitHub Actions workflow.

## 5. Recommended staged implementation plan

### Stage 1 — Native Android feasibility prototype
Build a minimal Android ARM64 APK with SDL3, an OpenGL ES context and a working Go-to-native call. Log the renderer/vendor/version and demonstrate a visible clear color. Do not link BASS or the full renderer unless required for this small proof.

**Exit criteria:** the APK installs, opens, renders a visible frame and successfully invokes a Go test function. If the architecture fails, stop and reconsider before porting other features.

### Stage 2 — GLES renderer port
Replace desktop-only GL assumptions with an Android-compatible GLES path or an explicit abstraction. Adapt batch/base-instance usage, texture and framebuffer operations, and shaders. Validate individual renderer subsystems incrementally.

**Exit criteria:** an existing beatmap scene renders on GLES without relying on unsupported desktop extensions.

### Stage 3 — Android storage and map import
Add private application paths, asset extraction and a document-picker flow for `.osz` imports. Expose a way to list imported beatmaps and difficulties as stable local entries, using local metadata/MD5 rather than online IDs.

**Exit criteria:** a real `.osz` can be imported and its map/difficulty list can be displayed.

### Stage 4 — Android map-selection UI
Implement a native Android menu to search and choose a song/difficulty and start gameplay. Keep this UI separate from the desktop ImGui launcher. Pass the local map identity into the game safely.

**Exit criteria:** the selected difficulty—not just a hard-coded map—reaches the gameplay loading path.

### Stage 5 — Touch gameplay and lifecycle
Implement reliable touch press/release events, coordinate transforms and multi-touch behavior for circles and sliders. Add pause/resume, focus handling, audio resynchronization and a configurable offset if needed.

**Exit criteria:** a real map can be played on-device, with measurable input response and audio synchronization.

### Stage 6 — CI, validation and release readiness
Use a separate Android workflow. Build for `arm64-v8a`, inspect APK contents, run static checks and add parser/import tests. Run an emulator smoke test if practical, but reserve real-device checks for GPU support, audio latency, touch behavior, performance, heat and battery.

## 6. Testing strategy

### GitHub Actions
- Run Go static checks and any available tests.
- Add focused tests for parsing local maps and importing a synthetic `.osz`.
- Cross-compile Android-compatible Go pieces with the NDK toolchain.
- Run Gradle `assembleDebug`.
- Inspect that the APK contains the expected `arm64-v8a` libraries and manifest declarations.
- Validate GLES shaders with an appropriate offline tool when possible.
- Run an Android emulator startup smoke test only if the runner configuration supports it.

### Physical Android device
- Verify GPU/GLES capabilities and rendering.
- Test import with real archives from local storage/providers.
- Test touch timing, slider following and multiple fingers.
- Measure audio synchronization and latency.
- Test pause/resume, orientation/surface changes, performance, thermal behavior and battery.

Compilation alone is not proof of a playable Android port.

## 7. Complexity and risk

| Area | Complexity | Main risk |
|---|---|---|
| Go/SDL/JNI feasibility prototype | Medium | Toolchain and native integration |
| OpenGL-to-GLES renderer port | High | Widespread API and shader incompatibility |
| Native BASS/audio integration | Medium–High | ARM64 binaries, licensing and synchronization |
| Android paths and `.osz` import | Medium | Storage model, safe extraction and errors |
| Song/difficulty UI | Medium | Connecting Android selection to existing game startup |
| Touch, sliders and lifecycle | High | Accurate input, multi-touch and pause/resume |
| CI and artifact generation | Low–Medium | Reproducible SDK/NDK/native dependencies |

## 8. Audit limitations and unresolved items

- This was a static audit of the supplied project context; no source build, APK build or device run was performed.
- Some dependencies were inspected at their repository HEAD rather than being verified against the exact versions pinned in `go.mod`.
- The full launcher song-selection flow, file-map path lookup and the precise online API decision in `player.go` still need deeper tracing.
- SDL3/purego compatibility on the exact pinned versions is unverified.
- Android availability and redistributable licensing of the required BASS libraries are unverified.
- The suspected processed-object slicing bug needs upstream comparison and a regression test.
- OpenGL/GLES compatibility must be measured on an actual GLES context; extension availability must not be guessed.

## Overall recommendation

Start with the native Android feasibility prototype, but keep it separate from the existing gameplay code. The renderer's dependence on desktop OpenGL is the dominant technical risk. Once the Go/SDL/GLES foundation is demonstrably working, port the renderer and then build the Android map library, song selector and touch gameplay around the reusable beatmap/ruleset core.

## Stage 3 follow-up — 2026-10-20

The requested `android/port-stage2` branch was not present in the repository when this stage was prepared. The isolated `android/port-stage3` branch was therefore based on the available `launcher` branch, which contains the Stage 2 Android project and this audit. No changes were made to `master`.

Stage 3 now extracts the existing `QuadBatch` GPU operations behind a `spriteBatchRenderer` interface. The desktop implementation is retained in `framework/graphics/batch/renderer_desktop.go`; `framework/graphics/batch/renderer_android.go` provides a GLES 3 implementation that uploads the game's existing 13-float sprite-instance records at offset zero and calls `glDrawElementsInstanced`, avoiding desktop base-instance semantics. The public `QuadBatch` API and CPU-side transform/packing behavior remain in `batch.go`.

The Android nested Go module now imports the root `framework/graphics/batch` package, and the C ABI exposes `TaktRendererInit/Draw/Destroy`. The Android smoke application constructs a deterministic GLES texture, then calls the real `QuadBatch.DrawTexture` path and verifies a sampled framebuffer pixel. This is a connected renderer slice rather than a separate C-only triangle renderer, but it is still a **test fixture, not a real beatmap hit object or playable gameplay scene**.

The fixture currently owns a narrow GLES texture implementation. The existing texture/atlas system still calls desktop DSA APIs, and the rest of the root GL loader, VAO/VBO/IBO, framebuffer and renderer packages have not been made Android-compatible as a whole. The entire desktop game/launcher is not linked into Android. The new backend has not yet been cross-compiled or runtime-tested; no success claim is made before CI evidence is available.

Runtime validation is assigned to the branch's GitHub Actions workflow: the x86_64 emulator smoke job runs automatically on pushes to `android/port-stage3` and now checks Go/JNI, GLES context, diagnostic pixels, real sprite shader compilation, the `QuadBatch` fixture's pixel, and another fixture pixel after background/foreground. Until that job reports success, checks remain pending. An emulator pass does not prove the ARM64 APK on a physical phone, and the game is not yet playable.

