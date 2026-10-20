// Own module so the root TaktChase module (go build ./...) never sees this
// experiment. scripts/gomobile-probe.sh copies it to a temp dir and adds the
// pinned golang.org/x/mobile requirement there.
module github.com/shntsk/taktchase/android/gomobile-probe

go 1.27.0
