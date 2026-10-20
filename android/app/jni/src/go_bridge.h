#ifndef GO_BRIDGE_H
#define GO_BRIDGE_H

/* Calls the Go test functions and logs the results to logcat.
 * Returns 1 on success, 0 if any result is wrong. */
int takt_go_selftest(void);

#endif /* GO_BRIDGE_H */
