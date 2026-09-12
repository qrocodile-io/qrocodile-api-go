// Command changelog-section prints one version's section of CHANGELOG.md, for use as GitHub
// release notes.
//
//	go run ./cmd/changelog-section 0.2.0
//
// Exits non-zero when that version has no section, or has one with no content. The release
// workflow runs this before creating the GitHub release, so a release with nothing to say about
// it fails while that is still fixable.
//
// Keeping the changelog authoritative is the point. The release notes are a copy of one
// section, never a second thing to write and keep in step. Mirrors
// qrocodile-api-node's scripts/changelog-section.mjs.
package main

import (
	"fmt"
	"os"
	"strings"
)

const changelogPath = "CHANGELOG.md"

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/changelog-section <version>")
		os.Exit(2)
	}
	version := os.Args[1]

	data, err := os.ReadFile(changelogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	section, ok := extractSection(string(data), version)
	if !ok {
		fmt.Fprintf(os.Stderr, `%s has no content under "## %s".

Every release needs to say what changed — the GitHub release notes are this section. Add the
entry, then move the tag:

  # edit %s
  git commit -am "Changelog for %s"
  git tag -fa v%s -m "%s" && git push --force origin v%s

Nothing has been published at this point (a Go module needs no publish step beyond the tag), so
re-tagging is safe.
`, changelogPath, version, changelogPath, version, version, version, version)
		os.Exit(1)
	}

	fmt.Println(section)
}

// extractSection returns everything under "## <version>", up to the next "## " heading or the
// end of the file, trimmed. The second return value is false when that heading doesn't exist or
// its section is empty. Headings are matched on the exact version so "0.1.0" cannot match
// "0.1.0-beta.1" — a pre-release has its own section or none at all.
func extractSection(markdown, version string) (string, bool) {
	lines := strings.Split(markdown, "\n")

	start := -1
	heading := "## " + version
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i
			break
		}
	}
	if start == -1 {
		return "", false
	}

	rest := lines[start+1:]
	end := len(rest)
	for i, line := range rest {
		if strings.HasPrefix(line, "## ") {
			end = i
			break
		}
	}

	body := strings.TrimSpace(strings.Join(rest[:end], "\n"))
	if body == "" {
		return "", false
	}
	return body, true
}
