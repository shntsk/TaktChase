/* Hand-written C ABI of libtaktgo.so (Go, -buildmode=c-shared).
 * Keep in sync with android/go/probe_export.go. Only scalar C types and plain
 * pointers are used: no Go strings, slices, interfaces or callbacks. */
#ifndef TAKTGO_API_H
#define TAKTGO_API_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

int32_t TaktProbeAdd(int32_t a, int32_t b);
int64_t TaktProbeSumParallel(int32_t n);
/* Writes a NUL-terminated string into buf; returns its length or -1. */
int32_t TaktProbeDescribe(char *buf, int32_t capacity);

#ifdef __cplusplus
}
#endif

#endif /* TAKTGO_API_H */
