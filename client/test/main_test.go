package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// testVersion is injected through -ldflags exactly as a distributed build
// injects its own, so the version test exercises the real injection path.
const testVersion = "v9.9.9-test"

// feedBinary is the one binary the whole suite runs; TestMain compiles it once
// from the module root, because the process boundary is the only seam.
var feedBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "feed-e2e-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating build directory: %v\n", err)
		os.Exit(1)
	}

	name := "feed"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	feedBinary = filepath.Join(dir, name)

	build := exec.Command(
		"go", "build", "-trimpath",
		"-ldflags", "-X main.version="+testVersion,
		"-o", feedBinary,
		"./cmd/feed",
	)
	build.Dir = ".."
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "building feed: %v\n", err)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
