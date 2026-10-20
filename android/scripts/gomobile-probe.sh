#!/usr/bin/env bash
# Experiment B: can gomobile/gobind package the probe for android/arm64?
# Results are informational (the CI job is allowed to fail). Everything is done
# in a temp copy so the repository is never modified.
#
# Needs: go, ANDROID_HOME and ANDROID_NDK_HOME. Output: $OUT_DIR (default /tmp/gomobile-out).
set -euo pipefail

# Pinned golang/mobile commit (HEAD when this stage was written).
GOMOBILE_COMMIT=19777044486779222400e9ca16c4306f9c6dbfa9
api=24

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
: "${ANDROID_NDK_HOME:?ANDROID_NDK_HOME required}"
: "${ANDROID_HOME:?ANDROID_HOME required}"
out="${OUT_DIR:-/tmp/gomobile-out}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$out"

export GOTOOLCHAIN=local
export GOBIN="$work/bin"
export PATH="$GOBIN:$PATH"

echo "Go: $(go env GOVERSION) | gomobile commit: $GOMOBILE_COMMIT"
mkdir -p "$work/mod"
cp -r "$root/gomobile-probe/." "$work/mod/"
cd "$work/mod"

go get "golang.org/x/mobile@${GOMOBILE_COMMIT}"
go install "golang.org/x/mobile/cmd/gomobile@${GOMOBILE_COMMIT}" \
           "golang.org/x/mobile/cmd/gobind@${GOMOBILE_COMMIT}"
go get "golang.org/x/mobile/bind@${GOMOBILE_COMMIT}"

gomobile init
gomobile bind -v -target=android/arm64 -androidapi "$api" -o "$out/taktprobe.aar" ./taktprobe

echo "== AAR contents"
unzip -l "$out/taktprobe.aar"
echo "== AAR native libraries"
unzip -Z1 "$out/taktprobe.aar" | grep '\.so$'
echo "gomobile bind succeeded; see android/README.md for why the main prototype still uses c-shared."
