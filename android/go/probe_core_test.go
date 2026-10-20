package main

import (
	"strings"
	"testing"
)

func TestAdd(t *testing.T) {
	if got := add(20, 22); got != 42 {
		t.Fatalf("add(20,22) = %d, want 42", got)
	}
}

func TestSumParallel(t *testing.T) {
	const n = 100000
	want := int64(n) * (n + 1) / 2
	for _, workers := range []int{1, 3, 4, 8} {
		if got := sumParallel(n, workers); got != want {
			t.Fatalf("sumParallel(%d, %d) = %d, want %d", n, workers, got, want)
		}
	}
	if sumParallel(0, 4) != 0 || sumParallel(10, 0) != 0 {
		t.Fatal("expected 0 for empty input")
	}
}

func TestCopyToBuffer(t *testing.T) {
	buf := make([]byte, 8)
	if n := copyToBuffer(buf, "hello"); n != 5 || string(buf[:5]) != "hello" || buf[5] != 0 {
		t.Fatalf("unexpected result n=%d buf=%v", n, buf)
	}
	if n := copyToBuffer(buf, "0123456789"); n != 7 || buf[7] != 0 {
		t.Fatalf("truncation failed n=%d buf=%v", n, buf)
	}
	if n := copyToBuffer(nil, "x"); n != -1 {
		t.Fatalf("empty buffer should return -1, got %d", n)
	}
}

func TestDescribe(t *testing.T) {
	d := describe()
	for _, part := range []string{"go=", "os=", "arch=", "cpus="} {
		if !strings.Contains(d, part) {
			t.Fatalf("describe() = %q, missing %q", d, part)
		}
	}
}
