package scripts_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstaller(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the POSIX installer requires sh and Unix utilities")
	}
	for _, tc := range []struct {
		name      string
		os        string
		arch      string
		tool      string
		checksum  string
		hashFails bool
		wantError string
	}{
		{name: "macOS Apple silicon sha256sum", os: "Darwin", arch: "arm64", tool: "sha256sum"},
		{name: "macOS Intel sha256sum", os: "Darwin", arch: "x86_64", tool: "sha256sum"},
		{name: "Linux sha256sum", os: "Linux", arch: "x86_64", tool: "sha256sum"},
		{name: "macOS shasum fallback", os: "Darwin", arch: "arm64", tool: "shasum"},
		{name: "Linux shasum fallback", os: "Linux", arch: "aarch64", tool: "shasum"},
		{name: "binary checksum marker", os: "Darwin", arch: "arm64", tool: "sha256sum", checksum: "binary"},
		{name: "sha256sum mismatch", os: "Darwin", arch: "arm64", tool: "sha256sum", checksum: "mismatch", wantError: "Checksum verification failed"},
		{name: "shasum mismatch", os: "Darwin", arch: "arm64", tool: "shasum", checksum: "mismatch", wantError: "Checksum verification failed"},
		{name: "missing checksum", os: "Darwin", arch: "arm64", tool: "sha256sum", checksum: "missing", wantError: "No checksum was published"},
		{name: "missing checksum tool", os: "Darwin", arch: "arm64", wantError: "Neither sha256sum nor shasum is available"},
		{name: "sha256sum command failure", os: "Darwin", arch: "arm64", tool: "sha256sum", hashFails: true, wantError: "Failed to calculate SHA-256 checksum"},
		{name: "shasum command failure", os: "Darwin", arch: "arm64", tool: "shasum", hashFails: true, wantError: "Failed to calculate SHA-256 checksum"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "installer with spaces")
			commands := filepath.Join(root, "commands")
			release := filepath.Join(root, "release")
			binDir := filepath.Join(root, "bin")
			tmpDir := filepath.Join(root, "tmp")
			for _, dir := range []string{commands, release, binDir, tmpDir} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			// Isolate PATH so host checksum tools and cosign cannot affect a case.
			for _, name := range []string{"awk", "cp", "gzip", "install", "mkdir", "mktemp", "rm", "tar"} {
				path, err := exec.LookPath(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path, filepath.Join(commands, name)); err != nil {
					t.Fatal(err)
				}
			}
			writeScript := func(name, script string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(commands, name), []byte("#!/bin/sh\nset -eu\n"+script), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			writeScript("uname", `case "$1" in
  -s) printf '%s\n' "$NEXRENDER_TEST_OS" ;;
  -m) printf '%s\n' "$NEXRENDER_TEST_ARCH" ;;
  *) exit 1 ;;
esac
`)
			writeScript("curl", `while [ "$#" -gt 0 ]; do
  case "$1" in
    https://*) url="$1" ;;
    --output) shift; output="$1" ;;
  esac
  shift
done
cp "$NEXRENDER_TEST_RELEASE/${url##*/}" "$output"
`)
			if tc.tool != "" {
				writeScript(tc.tool, `exec "$NEXRENDER_TEST_EXECUTABLE" -test.run=TestInstallerChecksumCommand -- `+tc.tool+` "$@"
`)
			}
			platform := strings.ToLower(tc.os)
			arch := "arm64"
			if tc.arch == "x86_64" {
				arch = "amd64"
			}
			archive := "nexrender_" + platform + "_" + arch + ".tar.gz"
			binary := []byte("#!/bin/sh\nprintf 'test nexrender\\n'\n")
			var buffer bytes.Buffer
			gz := gzip.NewWriter(&buffer)
			tw := tar.NewWriter(gz)
			if err := tw.WriteHeader(&tar.Header{Name: "nexrender", Mode: 0o755, Size: int64(len(binary))}); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write(binary); err != nil {
				t.Fatal(err)
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			if err := gz.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(release, archive), buffer.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256(buffer.Bytes()))
			filename := archive
			switch tc.checksum {
			case "binary":
				filename = "*" + archive
			case "mismatch":
				digest = strings.Repeat("0", 64)
			case "missing":
				filename = "another-archive.tar.gz"
			}
			checksums := fmt.Sprintf("%s  unrelated.tar.gz\n%s  %s\n", strings.Repeat("1", 64), digest, filename)
			if err := os.WriteFile(filepath.Join(release, "checksums.txt"), []byte(checksums), 0o644); err != nil {
				t.Fatal(err)
			}
			// Failures must preserve an existing install and leave no downloads behind.
			installed := filepath.Join(binDir, "nexrender")
			previous := []byte("previous install")
			if err := os.WriteFile(installed, previous, 0o755); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("sh", "install.sh")
			cmd.Env = []string{
				"PATH=" + commands,
				"TMPDIR=" + tmpDir,
				"NEXRENDER_BIN_DIR=" + binDir,
				"NEXRENDER_SKIP_SETUP=1",
				"NEXRENDER_TEST_RELEASE=" + release,
				"NEXRENDER_TEST_OS=" + tc.os,
				"NEXRENDER_TEST_ARCH=" + tc.arch,
				"NEXRENDER_TEST_EXECUTABLE=" + executable,
				fmt.Sprintf("NEXRENDER_TEST_HASH_FAILS=%t", tc.hashFails),
			}
			output, runErr := cmd.CombinedOutput()
			wantBinary := binary
			if tc.wantError != "" {
				wantBinary = previous
				if runErr == nil || !strings.Contains(string(output), tc.wantError) {
					t.Fatalf("expected %q, got error %v and output:\n%s", tc.wantError, runErr, output)
				}
			} else if runErr != nil || !strings.Contains(string(output), "Installed nexrender") {
				t.Fatalf("installer failed: %v\n%s", runErr, output)
			}
			gotBinary, err := os.ReadFile(installed)
			if err != nil || !bytes.Equal(gotBinary, wantBinary) {
				t.Fatalf("installed binary = %q, error %v; want %q", gotBinary, err, wantBinary)
			}
			entries, err := os.ReadDir(tmpDir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("temporary downloads were not cleaned up: %v, %v", entries, err)
			}
		})
	}
}

// The test process acts as a checksum tool, hashing the actual downloaded bytes.
// Apple's sha256sum requires an explicit operand in --check mode, even for stdin.
func TestInstallerChecksumCommand(t *testing.T) {
	if os.Getenv("NEXRENDER_TEST_EXECUTABLE") == "" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	args = args[1:]
	tool, args := args[0], args[1:]
	if tool == "shasum" {
		if len(args) != 3 || args[0] != "-a" || args[1] != "256" {
			fmt.Fprintln(os.Stderr, "shasum must select SHA-256 with -a 256")
			os.Exit(1)
		}
		args = args[2:]
	}
	if tool == "sha256sum" && os.Getenv("NEXRENDER_TEST_OS") == "Darwin" && len(args) == 2 && args[0] == "--check" && args[1] == "--status" {
		fmt.Fprintln(os.Stderr, "usage: sha256sum [-bctwz] [files ...]")
		os.Exit(1)
	}
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(os.Stderr, "expected a single archive filename")
		os.Exit(1)
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%x  %s\n", sha256.Sum256(data), args[0])
	// A tool can print a digest and still fail. The installer must honor its exit.
	if os.Getenv("NEXRENDER_TEST_HASH_FAILS") == "true" {
		os.Exit(1)
	}
	os.Exit(0)
}
