#!/usr/bin/env bash
# Verify the debug APK contains exactly the expected native libraries for one
# ABI and that they are wired together. Exits non-zero with a clear message on
# the first problem, so CI never publishes an incomplete APK as functional.
#
# Usage: scripts/verify-apk.sh <apk> [abi]   (ANDROID_NDK_HOME / ANDROID_HOME optional)
set -euo pipefail

apk="${1:?usage: verify-apk.sh <apk> [abi]}"
abi="${2:-arm64-v8a}"
pkg="io.github.shntsk.taktchase.probe"

fail() { echo "VERIFY FAILED: $*" >&2; exit 1; }

[[ -f "$apk" ]] || fail "APK not found: $apk"

case "$abi" in
    arm64-v8a) machine_re='AArch64'; need_16k=1 ;;
    x86_64)    machine_re='Advanced Micro Devices X86-64'; need_16k=0 ;;
    *) fail "unsupported ABI $abi" ;;
esac

readelf_bin=""
if [[ -n "${ANDROID_NDK_HOME:-}" && -x "$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/bin/llvm-readelf" ]]; then
    readelf_bin="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/bin/llvm-readelf"
elif command -v readelf >/dev/null 2>&1; then
    readelf_bin="$(command -v readelf)"
else
    fail "no readelf available"
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
unzip -q "$apk" -d "$work/apk"

echo "== APK: $apk ($(du -h "$apk" | cut -f1))"
# Capture the listing once: piping unzip into `grep -q` would trip pipefail (SIGPIPE).
entries="$(unzip -Z1 "$apk")"
native="$(grep '^lib/.' <<<"$entries" | grep -v '/$' || true)"
echo "== native entries:"
# shellcheck disable=SC2001
sed 's/^/   /' <<<"$native"

# 1. Exactly the expected libraries, only for the expected ABI.
expected=("lib/$abi/libSDL3.so" "lib/$abi/libmain.so" "lib/$abi/libtaktgo.so")
for e in "${expected[@]}"; do
    grep -qxF "$e" <<<"$entries" || fail "missing $e in APK"
done
unexpected="$(grep -vxF "$(printf '%s\n' "${expected[@]}")" <<<"$native" || true)"
[[ -z "$unexpected" ]] || fail "unexpected native entries (other ABIs or third-party libs): $unexpected"
grep -qxF 'classes.dex' <<<"$entries" || fail "classes.dex missing"

# 2. Each library is an ELF for the right machine.
for e in "${expected[@]}"; do
    f="$work/apk/$e"
    "$readelf_bin" -h "$f" | grep -q "Machine:.*${machine_re}" || fail "$e is not built for $abi"
    echo "   $e: machine OK"
done

# 3. Dependency wiring of libmain.so.
needed="$("$readelf_bin" -d "$work/apk/lib/$abi/libmain.so" | sed -n 's/.*(NEEDED).*\[\(.*\)\]/\1/p')"
echo "== libmain.so NEEDED: $(echo "$needed" | tr '\n' ' ')"
for dep in libtaktgo.so libSDL3.so libGLESv3.so; do
    echo "$needed" | grep -qx "$dep" || fail "libmain.so does not depend on $dep"
done
echo "$needed" | grep -q '/' && fail "libmain.so has a path in NEEDED (Android cannot resolve it): $needed"

# 4. Go library: SONAME and exported C ABI.
go_dyn="$("$readelf_bin" -d "$work/apk/lib/$abi/libtaktgo.so")"
echo "$go_dyn" | grep -q 'SONAME.*\[libtaktgo.so\]' || fail "libtaktgo.so has no SONAME libtaktgo.so"
go_syms="$("$readelf_bin" --dyn-syms -W "$work/apk/lib/$abi/libtaktgo.so")"
for sym in TaktProbeAdd TaktProbeSumParallel TaktProbeDescribe; do
    echo "$go_syms" | grep -qw "$sym" || fail "libtaktgo.so does not export $sym"
done
echo "== libtaktgo.so exports: TaktProbeAdd TaktProbeSumParallel TaktProbeDescribe"

# 5. libmain.so exports SDL_main and the JNI entry points.
main_syms="$("$readelf_bin" --dyn-syms -W "$work/apk/lib/$abi/libmain.so")"
for sym in SDL_main \
    Java_io_github_shntsk_taktchase_probe_GoBridge_nativeAdd \
    Java_io_github_shntsk_taktchase_probe_GoBridge_nativeDescribe; do
    echo "$main_syms" | grep -qw "$sym" || fail "libmain.so does not export $sym"
done
echo "== libmain.so exports: SDL_main + 2 JNI functions"

# 6. 16 KB page alignment of LOAD segments (required on Android 15+ for arm64).
if [[ "$need_16k" == 1 ]]; then
    for e in "${expected[@]}"; do
        while read -r align; do
            [[ "$align" =~ ^0x[0-9a-fA-F]+$ ]] || fail "cannot parse LOAD alignment '$align' in $e"
            (( align >= 16384 )) || fail "$e has LOAD segment alignment $align (< 16 KB)"
        done < <("$readelf_bin" -lW "$work/apk/$e" | awk '$1=="LOAD" {print $NF}')
    done
    echo "== LOAD segment alignment >= 16 KB: OK"
fi

# 7. Manifest facts via aapt2 when available.
aapt2="$(find "${ANDROID_HOME:-${ANDROID_SDK_ROOT:-/nonexistent}}/build-tools" -name aapt2 -type f 2>/dev/null | sort | tail -1 || true)"
if [[ -n "$aapt2" ]]; then
    badging="$("$aapt2" dump badging "$apk")"
    echo "$badging" | grep -q "^package: name='$pkg'" || fail "unexpected package name"
    echo "$badging" | grep -q "native-code: '$abi'$" || fail "manifest native-code is not exactly '$abi'"
    echo "$badging" | grep -q "uses-gl-es: '0x30000'" || fail "GLES 3.0 feature missing from manifest"
    echo "== aapt2 badging: package/native-code/GLES OK"
else
    echo "== aapt2 not found; skipped badging check"
fi

echo "VERIFY OK: $apk has the expected $abi libraries"
