/* Native bridge between Android (Java/JNI and the SDL_main thread) and the Go
 * library. Everything that crosses into Go goes through taktgo_api.h. */
#include <jni.h>
#include <stdint.h>

#include "go_bridge.h"
#include "takt_log.h"
#include "taktgo_api.h"

int takt_go_selftest(void) {
    char desc[160] = {0};
    int32_t add = TaktProbeAdd(20, 22);
    int64_t sum = TaktProbeSumParallel(1000);
    int32_t len = TaktProbeDescribe(desc, (int32_t)sizeof(desc));

    TLOGI("TAKT_PROBE go_add=%d (expected 42)", (int)add);
    TLOGI("TAKT_PROBE go_sum=%lld (expected 500500)", (long long)sum);
    TLOGI("TAKT_PROBE go_describe[%d]=%s", (int)len, desc);

    if (add != 42 || sum != 500500 || len <= 0) {
        TLOGE("TAKT_PROBE go_selftest=FAIL");
        return 0;
    }
    TLOGI("TAKT_PROBE go_selftest=PASS");
    return 1;
}

/* JNI: io.github.shntsk.taktchase.probe.GoBridge.nativeAdd(int, int) */
JNIEXPORT jint JNICALL
Java_io_github_shntsk_taktchase_probe_GoBridge_nativeAdd(JNIEnv *env, jclass clazz,
                                                         jint a, jint b) {
    (void)env;
    (void)clazz;
    jint r = (jint)TaktProbeAdd((int32_t)a, (int32_t)b);
    TLOGI("TAKT_PROBE jni_add(%d, %d)=%d", (int)a, (int)b, (int)r);
    return r;
}

/* JNI: io.github.shntsk.taktchase.probe.GoBridge.nativeDescribe() */
JNIEXPORT jstring JNICALL
Java_io_github_shntsk_taktchase_probe_GoBridge_nativeDescribe(JNIEnv *env, jclass clazz) {
    (void)clazz;
    char buf[160] = {0};
    if (TaktProbeDescribe(buf, (int32_t)sizeof(buf)) < 0) {
        return (*env)->NewStringUTF(env, "error");
    }
    TLOGI("TAKT_PROBE jni_describe=%s", buf);
    return (*env)->NewStringUTF(env, buf);
}
