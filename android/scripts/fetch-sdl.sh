#!/usr/bin/env bash
# Fetch SDL3 sources at the commit pinned in android/SDL_VERSION into
# android/app/jni/SDL. Fails if the fetched commit does not match the pin.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# Provides: tag, commit, repo (pinned SDL release).
tag="" commit="" repo=""
# shellcheck disable=SC1091
source "$root/SDL_VERSION"
: "${tag:?SDL_VERSION must define tag}" "${commit:?SDL_VERSION must define commit}" "${repo:?SDL_VERSION must define repo}"
dest="$root/app/jni/SDL"

if [[ -d "$dest/.git" ]] && [[ "$(git -C "$dest" rev-parse HEAD)" == "$commit" ]]; then
    echo "SDL $tag ($commit) already present in $dest"
    exit 0
fi

rm -rf "$dest"
mkdir -p "$dest"
git -C "$dest" init -q
git -C "$dest" remote add origin "$repo"
git -C "$dest" fetch -q --depth 1 origin "$commit"
git -C "$dest" checkout -q FETCH_HEAD

actual="$(git -C "$dest" rev-parse HEAD)"
if [[ "$actual" != "$commit" ]]; then
    echo "ERROR: SDL checkout is $actual, expected $commit" >&2
    exit 1
fi
for f in CMakeLists.txt android-project/app/src/main/java/org/libsdl/app/SDLActivity.java; do
    [[ -f "$dest/$f" ]] || { echo "ERROR: $f missing from SDL checkout" >&2; exit 1; }
done
echo "SDL $tag ($actual) ready in $dest"
