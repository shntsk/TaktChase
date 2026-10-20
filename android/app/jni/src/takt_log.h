#ifndef TAKT_LOG_H
#define TAKT_LOG_H

#include <android/log.h>

#define TAKT_TAG "TaktChase"
#define TLOGI(...) __android_log_print(ANDROID_LOG_INFO, TAKT_TAG, __VA_ARGS__)
#define TLOGW(...) __android_log_print(ANDROID_LOG_WARN, TAKT_TAG, __VA_ARGS__)
#define TLOGE(...) __android_log_print(ANDROID_LOG_ERROR, TAKT_TAG, __VA_ARGS__)

#endif /* TAKT_LOG_H */
