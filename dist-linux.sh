#!/bin/bash
set -eo pipefail
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=1
export CC=gcc
export CXX=g++

# Check the native development dependencies needed by Gio/cgo before compiling.
# This keeps local builds and release builds aligned and reports the actual
# package names instead of failing deep inside the Go compiler.
if ! command -v pkg-config >/dev/null 2>&1; then
  echo "Missing native dependency: pkg-config" >&2
  echo "Arch: sudo pacman -S --needed pkgconf" >&2
  echo "Debian/Ubuntu: sudo apt install pkg-config" >&2
  exit 1
fi

missing_modules=()
for module in gl xkbcommon wayland-client gtk+-3.0 glib-2.0 x11-xcb xkbcommon-x11; do
  if ! pkg-config --exists "$module"; then
    missing_modules+=("$module")
  fi
done

if ! printf '%s\\n' '#include <vulkan/vulkan.h>' | "$CC" -E -x c - >/dev/null 2>&1; then
  missing_modules+=("header vulkan/vulkan.h")
fi

if (( ${#missing_modules[@]} > 0 )); then
  echo "Missing Linux build dependencies: ${missing_modules[*]}" >&2
  echo >&2
  echo "Arch Linux:" >&2
  echo "  sudo pacman -S --needed base-devel pkgconf libx11 libxrandr libxcursor libxinerama libxi libxext libxxf86vm libxcb libxkbcommon libxkbcommon-x11 wayland wayland-protocols mesa libglvnd gtk3 alsa-lib vulkan-headers" >&2
  echo "Debian/Ubuntu:" >&2
  echo "  sudo apt install build-essential pkg-config libx11-dev libx11-xcb-dev libxrandr-dev libxcursor-dev libxinerama-dev libxi-dev libxkbcommon-dev libxkbcommon-x11-dev libxext-dev libxxf86vm-dev libgl1-mesa-dev libegl1-mesa-dev libgtk-3-dev libasound2-dev libwayland-dev wayland-protocols libvulkan-dev" >&2
  exit 1
fi
export BUILD_DIR=./dist/build-linux
export TARGET_DIR=./dist/artifacts

exec=$1
build=$1
if [[ $# > 1 ]]
then
  exec+='-s'$2
  build+='-snapshot'$2
fi

mkdir -p $BUILD_DIR

go run tools/assets/assets.go ./ $BUILD_DIR/

go build -trimpath -ldflags "-s -w -X 'github.com/shntsk/taktchase/build.VERSION=$build' -X 'github.com/shntsk/taktchase/build.Stream=Release'" -buildmode=c-shared -o $BUILD_DIR/taktchase-core.so -v -x -tags "exclude_cimgui_glfw exclude_cimgui_sdli"

mv $BUILD_DIR/taktchase-core.so $BUILD_DIR/libtaktchase-core.so
cp {libbass.so,libbass_fx.so,libbassmix.so,libyuv.so,libSDL3.so} $BUILD_DIR/

gcc -no-pie --verbose -O3 -o $BUILD_DIR/taktchase-cli -I. cmain/main_taktchase.c -I$BUILD_DIR/ -Wl,-rpath,'$ORIGIN' -L$BUILD_DIR/ -ltaktchase-core

gcc -no-pie --verbose -O3 -D LAUNCHER -o $BUILD_DIR/taktchase -I. cmain/main_taktchase.c -I$BUILD_DIR/ -Wl,-rpath,'$ORIGIN' -L$BUILD_DIR/ -ltaktchase-core

rm $BUILD_DIR/taktchase-core.h

# Bundle FFmpeg into the package. Set SKIP_FFMPEG=1 to skip (the app falls back to ffmpeg from PATH).
if [[ -z "$SKIP_FFMPEG" ]]; then
  go run tools/ffmpeg/ffmpeg.go $BUILD_DIR/
fi

mkdir -p $TARGET_DIR

go run tools/pack2/pack.go $TARGET_DIR/taktchase-$exec-linux.zip $BUILD_DIR/

rm -rf $BUILD_DIR