//go:build (linux || darwin) && (native_module || debug)

package native_module

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNativeModuleLifecycle(t *testing.T) {
	cc := findCCompiler(t)
	tempDir := t.TempDir()

	sourcePath := filepath.Join(tempDir, "module.c")
	libraryPath := filepath.Join(tempDir, "module"+sharedLibraryExtension())
	source := `
#include <stdio.h>

char* hello(int argc, char** argv) {
	static char output[256];
	snprintf(output, sizeof(output), "argc=%d first=%s", argc, argc > 1 ? argv[1] : "");
	return output;
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(cc, "-shared", "-fPIC", sourcePath, "-o", libraryPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build test module: %s\n%s", err, string(output))
	}

	libraryBytes, err := os.ReadFile(libraryPath)
	if err != nil {
		t.Fatal(err)
	}

	module, err := loadNativeModule("test-module", libraryBytes)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupNativeModule(module)

	output, err := callNativeModule(module, "hello", []string{"world"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "argc=2 first=world") {
		t.Fatalf("unexpected module output: %q", output)
	}

	if err := closeNativeModule(module); err != nil {
		t.Fatal(err)
	}
}

func findCCompiler(t *testing.T) string {
	t.Helper()
	for _, compiler := range []string{"cc", "clang", "gcc"} {
		if path, err := exec.LookPath(compiler); err == nil {
			return path
		}
	}
	t.Skip("no C compiler available for native module smoke test")
	return ""
}

func sharedLibraryExtension() string {
	if runtime.GOOS == "darwin" {
		return ".dylib"
	}
	return ".so"
}
