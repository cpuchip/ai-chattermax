package main

import "testing"

func TestVersion(t *testing.T) {
	got := Version()
	want := "v0.1.0"
	if got != want {
		t.Errorf("Version() = %q, want %q", got, want)
	}
}
