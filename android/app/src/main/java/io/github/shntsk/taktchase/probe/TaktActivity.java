package io.github.shntsk.taktchase.probe;

import android.os.Bundle;
import android.util.Log;

import org.libsdl.app.SDLActivity;

/**
 * Activity of the stage-2 feasibility prototype. It only wires SDL3 to our
 * native libraries and exercises the Java -> JNI -> Go path once at start-up.
 * It does not start any TaktChase launcher or gameplay code.
 */
public class TaktActivity extends SDLActivity {
    private static final String TAG = "TaktChase";

    /** Load order: SDL3, then the Go library, then our SDL_main library. */
    @Override
    protected String[] getLibraries() {
        return new String[] {"SDL3", "taktgo", "main"};
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        Log.i(TAG, "TAKT_PROBE activity_onCreate recreated=" + (savedInstanceState != null));
        super.onCreate(savedInstanceState);
        probeGoFromJava();
    }

    private void probeGoFromJava() {
        try {
            int sum = GoBridge.nativeAdd(40, 2);
            String info = GoBridge.nativeDescribe();
            Log.i(TAG, "TAKT_PROBE java_jni_go add=" + sum + " describe=" + info);
            Log.i(TAG, "TAKT_PROBE java_jni_selftest=" + (sum == 42 ? "PASS" : "FAIL"));
        } catch (UnsatisfiedLinkError e) {
            // Happens if SDL failed to load the libraries; SDLActivity shows its own error.
            Log.e(TAG, "TAKT_PROBE java_jni_selftest=FAIL (native libraries not loaded)", e);
        }
    }

    @Override
    protected void onPause() {
        Log.i(TAG, "TAKT_PROBE activity_onPause");
        super.onPause();
    }

    @Override
    protected void onResume() {
        super.onResume();
        Log.i(TAG, "TAKT_PROBE activity_onResume");
    }

    @Override
    protected void onDestroy() {
        Log.i(TAG, "TAKT_PROBE activity_onDestroy finishing=" + isFinishing());
        super.onDestroy();
    }
}
