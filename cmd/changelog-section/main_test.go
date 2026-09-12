package main

import (
	"os/exec"
	"strings"
	"testing"
)

// This decides whether a release happens: the release workflow runs it before creating the
// GitHub release and fails when a version has nothing written about it. A wrong answer either
// blocks a good release or lets one out with empty notes, so the parsing is worth pinning down.
const testChangelog = `# Changelog

Preamble that belongs to no version.

## Unreleased

## 0.2.0

### Added

- A thing.

## 0.1.0

First public release.

### Notes

- Server-side only.
`

func TestExtractSection(t *testing.T) {
	t.Run("returns everything under the heading, up to the next one", func(t *testing.T) {
		got, ok := extractSection(testChangelog, "0.2.0")
		if !ok || got != "### Added\n\n- A thing." {
			t.Errorf("got %q, %v", got, ok)
		}
	})

	t.Run("reads the last section to the end of the file", func(t *testing.T) {
		got, ok := extractSection(testChangelog, "0.1.0")
		want := "First public release.\n\n### Notes\n\n- Server-side only."
		if !ok || got != want {
			t.Errorf("got %q, %v, want %q", got, ok, want)
		}
	})

	// An empty "## Unreleased" is the normal state between releases, not a section.
	t.Run("treats a heading with no content as missing", func(t *testing.T) {
		if _, ok := extractSection(testChangelog, "Unreleased"); ok {
			t.Error("expected no section for Unreleased")
		}
	})

	t.Run("returns nothing for a version that is not there", func(t *testing.T) {
		if _, ok := extractSection(testChangelog, "9.9.9"); ok {
			t.Error("expected no section for 9.9.9")
		}
	})

	// Exact-match on the heading. A prefix match would hand 0.1.0's notes to 0.1.0-beta.1,
	// which is precisely when someone is least likely to check.
	t.Run("does not let one version match another that starts the same way", func(t *testing.T) {
		if _, ok := extractSection(testChangelog, "0.1"); ok {
			t.Error("expected no section for 0.1")
		}
		if _, ok := extractSection("## 0.1.0-beta.1\n\nBeta notes.\n", "0.1.0"); ok {
			t.Error("expected no section for 0.1.0 to match 0.1.0-beta.1's heading")
		}
	})

	t.Run("ignores the preamble above the first version", func(t *testing.T) {
		if _, ok := extractSection(testChangelog, "Changelog"); ok {
			t.Error("expected no section for the preamble heading")
		}
	})
}

// The failure path is the one that matters — it is what stops a release going out with nothing
// written about it, so it has to actually exit non-zero. Runs the built binary rather than
// calling main() directly, since the behavior under test includes the process exit code.
func TestCLI_missingVersion(t *testing.T) {
	dir := t.TempDir()
	bin := dir + "/changelog-section"
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "9.9.9")
	cmd.Dir = "testdata"
	out, err := cmd.CombinedOutput()

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected an ExitError, got %v (output: %s)", err, out)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
	}
	if !strings.Contains(string(out), `has no content under "## 9.9.9"`) {
		t.Errorf("output missing expected message: %s", out)
	}
	if !strings.Contains(string(out), "git tag -fa") {
		t.Errorf("output missing re-tag instructions: %s", out)
	}
}

func TestCLI_noVersion(t *testing.T) {
	dir := t.TempDir()
	bin := dir + "/changelog-section"
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	cmd := exec.Command(bin)
	cmd.Dir = "testdata"
	err := cmd.Run()

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected an ExitError, got %v", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2", exitErr.ExitCode())
	}
}
