#!/usr/bin/env bash
# Smoke test on a running emulator/device (adb must be connected).
# Installs the APK, starts the Activity, then checks logcat for the markers
# written by the prototype. Fails with the captured log if anything is missing.
#
# Usage: scripts/emulator-smoke.sh <apk> [logfile]
set -uo pipefail

apk="${1:?usage: emulator-smoke.sh <apk> [logfile]}"
log="${2:-logcat-smoke.txt}"
pkg="io.github.shntsk.taktchase.probe"
act="$pkg/.TaktActivity"

fail() { echo "SMOKE FAILED: $*" >&2; echo "---- logcat ----" >&2; cat "$log" >&2 || true; exit 1; }
collect() { adb logcat -d -s TaktChase:V GoProbe:V AndroidRuntime:E DEBUG:V SDL:V SDL/APP:V >"$log" 2>&1; }

adb install -r "$apk" || { : >"$log"; fail "adb install failed"; }
adb logcat -c
adb shell am start -W -n "$act" || { collect; fail "could not start $act"; }
sleep 20
collect

for marker in \
    "TAKT_PROBE sdl_init=ok" \
    "Go runtime initialised inside libtaktgo.so" \
    "TAKT_PROBE go_selftest=PASS" \
    "TAKT_PROBE java_jni_selftest=PASS" \
    "TAKT_PROBE gles_major_minor=3" \
    "TAKT_PROBE scene_check=PASS" \
    "TAKT_RENDER sprite_shader_compile=PASS" \
    "TAKT_PROBE heartbeat"; do
    grep -qF "$marker" "$log" || fail "marker not found: $marker"
done
grep -q "FATAL EXCEPTION" "$log" && fail "Java crash detected"
adb shell pidof "$pkg" >/dev/null || fail "process is not running after 20 s"
echo "OK: start-up, Go calls (C and JNI), GLES scene and heartbeat verified"

# Lifecycle: background and foreground again must not crash the app.
adb shell input keyevent KEYCODE_HOME
sleep 4
adb shell am start -n "$act"
sleep 6
collect
for marker in "event=DID_ENTER_BACKGROUND" "event=DID_ENTER_FOREGROUND" "TAKT_PROBE resumed_render=PASS"; do
    grep -qF "$marker" "$log" || fail "lifecycle marker not found: $marker"
done
grep -q "FATAL EXCEPTION" "$log" && fail "crash after background/foreground"
adb shell pidof "$pkg" >/dev/null || fail "process died after background/foreground"
echo "OK: background/foreground cycle survived"
