package io.github.shntsk.taktchase.probe;

/**
 * JNI entry points implemented in libmain.so (app/jni/src/go_bridge.c), which
 * forward to the Go library libtaktgo.so through a plain C ABI.
 *
 * The libraries are loaded by {@link TaktActivity#getLibraries()} through SDL's
 * own loader, so no System.loadLibrary call is needed here.
 */
public final class GoBridge {
    private GoBridge() {}

    public static native int nativeAdd(int a, int b);

    public static native String nativeDescribe();
}
