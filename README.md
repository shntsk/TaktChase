<div align="center">
  <h1>TaktChase</h1>
  <p>A rhythm-game project focused on timing, precision, and control.</p>
  <p>
    <a href="https://github.com/shntsk/TaktChase">Source code</a>
    ·
    <a href="https://github.com/shntsk/TaktChase/issues">Issues</a>
  </p>
</div>

> **Status: early development.** TaktChase is being shaped from an existing desktop rhythm-game codebase. The project identity and gameplay-focused direction are being established; it is not yet a finished standalone release.

## About

TaktChase is an in-development rhythm game built around the idea of playing beatmaps with precise timing and responsive input.

The current source is based on [danser-go](https://github.com/Wieku/danser-go), originally created by Sebastian Krajewski ([@Wieku](https://github.com/Wieku)). It already contains a desktop gameplay path for osu!standard maps, alongside rendering, audio, beatmap, and legacy replay-visualization tools. Work is underway to make the gameplay experience the center of TaktChase rather than carrying forward the original project's identity.

## Current state

The repository is a **development codebase**, not a ready-to-install game.

- **Desktop foundation:** the inherited code uses SDL3 and OpenGL, with existing Windows and Linux build scripts.
- **Interactive play:** the codebase includes a <code>-play</code> mode for osu!standard beatmaps and keyboard/mouse input.
- **Gameplay adaptation:** the standalone TaktChase identity, interface, and gameplay-focused structure still need work.
- **Android:** no Android app or APK is available yet. Mobile support requires a dedicated port of the rendering, input, and audio paths.

The existing build scripts and some internal names still refer to danser-go. They will be updated alongside the code so that renaming the project does not silently break its native libraries or build pipeline.

## Direction

The immediate goal is to build on the existing playable foundation and gradually make TaktChase its own project:

- Keep beatmap loading and real-time gameplay at the core.
- Improve hit feedback, timing judgments, and gameplay presentation.
- Separate the game experience from tools that only serve replay rendering or video generation.
- Establish consistent TaktChase names for the app, configuration, native binaries, and release artifacts.
- Evaluate Android support after the gameplay and platform layers have been separated cleanly.

These are development goals, not a claim that every item is already implemented.

## Building from source

The project is being refactored, so the current build setup still reflects the inherited desktop codebase. There are no TaktChase-branded installers or Android packages in this refactor yet.

The current Go module targets Go 1.27 and pins the Go 1.27.2 toolchain. It uses a native C/C++ toolchain and OpenGL. Linux builds also require X11 development libraries and GTK 3 development files. Build scripts and release artifacts will be documented here again once they are aligned with the TaktChase identity.

## Credits and licensing

TaktChase is based on [danser-go](https://github.com/Wieku/danser-go), created by Sebastian Krajewski ([@Wieku](https://github.com/Wieku)) and its contributors. Their authorship notices and third-party credits are preserved in the source tree.

Unless a file says otherwise, the inherited source is distributed under the **GNU General Public License v3.0**. See [LICENSE](LICENSE) and [CREDITS.md](CREDITS.md) for the full license and third-party notices.

TaktChase is an independent project and is not affiliated with, endorsed by, or connected to osu! or ppy. Beatmaps, skins, audio, and other user-provided content may have their own license terms.
