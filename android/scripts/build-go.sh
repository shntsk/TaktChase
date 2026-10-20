#!/usr/bin/env bash
# Build the Go probe as an Android shared library (CGO, -buildmode=c-shared).
#
# Usage: ANDROID_NDK_HOME=/path/to/ndk scripts/build-go.sh [abi]
#   abi: arm64-v8a (default) or x86_64 (emulator smoke test only)
# Optional: TAKT_GO_VERSION=go1.27.2 makes the script fail on any other Go.
set -euo pipefail

abi="${1:-arm64-v8a}"
api=24
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

: "${ANDROID_NDK_HOME:?ANDROID_NDK_HOME must point to the Android NDK (r28 expected)}"

case "$abi" in
    arm64-v8a) goarch=arm64; triple="aarch64-linux-android${api}" ;;
    x86_64)    goarch=amd64; triple="x86_64-linux-android${api}" ;;
    *) echo "ERROR: unsupported ABI '$abi'" >&2; exit 1 ;;
esac

host_tag=linux-x86_64
cc="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/$host_tag/bin/${triple}-clang"
if [[ ! -x "$cc" ]]; then
    echo "ERROR: NDK compiler not found: $cc" >&2
    exit 1
fi

export GOTOOLCHAIN=local
actual_go="$(go env GOVERSION)"
if [[ -n "${TAKT_GO_VERSION:-}" && "$actual_go" != "$TAKT_GO_VERSION" ]]; then
    echo "ERROR: Go is $actual_go but $TAKT_GO_VERSION is required" >&2
    exit 1
fi
echo "Go: $actual_go | NDK clang: $cc | ABI: $abi (GOARCH=$goarch)"

out="$root/go/build/$abi"
dest="$root/app/src/main/jniLibs/$abi"
repo="$(cd "$root/.." && pwd)"
mkdir -p "$out" "$dest"

# Generate the Android Go shader source from the same desktop assets used by
# CMake's native GLES shader check. The root desktop shader files stay canonical.
python3 "$root/scripts/generate-gles-sprite-header.py" \
    "$out/taktchase_sprite_gles.h" \
    "$repo/framework/graphics/batch/renderer_shaders_gles_generated_android.go"

# -soname: without it the linker may record the build path as DT_NEEDED in
# libraries linked against this one, which Android cannot resolve at runtime.
(
    cd "$root/go"
    CGO_ENABLED=1 GOOS=android GOARCH="$goarch" CC="$cc" \
        go build -trimpath -buildmode=c-shared \
        -ldflags "-s -w -extldflags=-Wl,-soname,libtaktgo.so,-z,max-page-size=16384" \
        -o "$out/libtaktgo.so" .
)

cp "$out/libtaktgo.so" "$dest/libtaktgo.so"
ls -l "$dest/libtaktgo.so"
file "$dest/libtaktgo.so"
