package ansi

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestGoldenUnix(t *testing.T) {
	runGolden(t, "testdata/unix", "testdata/unix-expected")
}

// runGolden walks every *.bin under inputDir, runs it through Strip + LineDiscipline,
// joins emitted lines with "\n", and either compares to or updates the corresponding
// .txt file under expectedDir.
func runGolden(t *testing.T, inputDir, expectedDir string) {
	t.Helper()

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		t.Fatalf("read input dir %s: %v", inputDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".bin") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".bin")
		t.Run(name, func(t *testing.T) {
			inputPath := filepath.Join(inputDir, entry.Name())
			expectedPath := filepath.Join(expectedDir, name+".txt")
			runGoldenCase(t, inputPath, expectedPath)
		})
	}
}

func runGoldenCase(t *testing.T, inputPath, expectedPath string) {
	t.Helper()

	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read input: %v", err)
	}

	// Run the captured bytes through the discipline pipeline.
	var lines [][]byte
	ld := NewLineDiscipline(func(line []byte) {
		// emit may reuse the slice — copy.
		cp := make([]byte, len(line))
		copy(cp, line)
		lines = append(lines, cp)
	})
	if _, err := ld.Write(inputBytes); err != nil {
		t.Fatalf("LineDiscipline.Write: %v", err)
	}
	if err := ld.Close(); err != nil {
		t.Fatalf("LineDiscipline.Close: %v", err)
	}

	// Join with literal newlines + a trailing newline (POSIX text-file convention).
	var got bytes.Buffer
	for _, l := range lines {
		got.Write(l)
		got.WriteByte('\n')
	}

	if *update {
		if err := os.MkdirAll(filepath.Dir(expectedPath), 0o755); err != nil {
			t.Fatalf("mkdir expected: %v", err)
		}
		if err := os.WriteFile(expectedPath, got.Bytes(), 0o644); err != nil {
			t.Fatalf("write expected: %v", err)
		}
		t.Logf("updated %s (%d bytes)", expectedPath, got.Len())
		return
	}

	expected, err := os.ReadFile(expectedPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("expected file %s does not exist; run with -update to populate", expectedPath)
			return
		}
		t.Fatalf("read expected: %v", err)
	}

	if !bytes.Equal(got.Bytes(), expected) {
		t.Errorf("golden mismatch for %s\n--- expected (%d bytes) ---\n%s\n--- got (%d bytes) ---\n%s",
			inputPath, len(expected), expected, got.Len(), got.String())
	}
}
