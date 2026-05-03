//go:build !windows

package wrapper

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestMain(m *testing.M) {
	// Build the peek binary once before any signal integration tests run.
	// This ensures the testdata/peek binary is always in sync with current source.
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: mkdir testdata: %v\n", err)
		os.Exit(1)
	}

	// Use "github.com/Pankaj3112/peek/cmd/peek" as the import path — go build
	// resolves it via the module. The working directory for tests is the package
	// dir, but "go build <import-path>" works from anywhere as long as the module
	// is in the module cache or the workspace. We use the relative path form so
	// we don't need to hardcode the module path.
	build := exec.Command("go", "build", "-o", "testdata/peek", "../../cmd/peek")
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: go build failed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
