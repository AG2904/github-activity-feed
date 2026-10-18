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

// The whole suite runs compiled binaries: the process boundary is the only
// seam. idleFeedBinary is the same code built with a 300ms idle timeout, so
// the five-minute rule can be exercised without waiting five minutes.
var (
	feedBinary     string
	idleFeedBinary string
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "feed-e2e-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating build directory: %v\n", err)
		os.Exit(1)
	}

	base := "-X main.version=" + testVersion
	feedBinary = buildBinary(dir, "feed", base)
	idleFeedBinary = buildBinary(dir, "feed-idle", base+
		" -X github.com/AG2904/github-activity-feed/client/internal/cli.idleTimeoutSetting=300ms")

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func buildBinary(dir, name, ldflags string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(dir, name)

	build := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", binary, "./cmd/feed")
	build.Dir = ".."
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "building %s: %v\n", name, err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	return binary
}
