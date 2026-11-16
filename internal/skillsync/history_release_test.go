package skillsync

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"testing/fstest"
)

func TestOfficialReleaseHistoryMatchesExactGoFingerprints(t *testing.T) {
	var sources []struct{ Version, Commit string }
	data, err := os.ReadFile("history_sources.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &sources); err != nil {
		t.Fatal(err)
	}
	var catalog map[string][]string
	if err := json.Unmarshal(historicalFingerprintsJSON, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		t.Run(source.Version, func(t *testing.T) {
			// Shallow source checkouts can still run all offline catalog tests.
			if err := exec.Command("git", "cat-file", "-e", source.Commit+"^{commit}").Run(); err != nil {
				t.Skip("pinned historical Git object is unavailable in this checkout")
			}
			archive, err := exec.Command("git", "-C", "../..", "archive", source.Commit, "--", "skills/auth").Output()
			if err != nil {
				t.Fatal(err)
			}
			tree := fstest.MapFS{}
			reader := tar.NewReader(bytes.NewReader(archive))
			for {
				header, err := reader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if header.Typeflag == tar.TypeDir || header.Typeflag == tar.TypeXGlobalHeader {
					continue
				}
				if header.Typeflag != tar.TypeReg {
					t.Fatal("non-regular historical file")
				}
				name := strings.TrimPrefix(header.Name, "skills/auth/")
				content, err := io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				tree[name] = &fstest.MapFile{Data: content}
			}
			original := tree["SKILL.md"].Data
			for _, version := range []string{"", source.Version, "v" + source.Version} {
				tree["SKILL.md"] = &fstest.MapFile{Data: RewriteVersion(original, version)}
				digest, err := treeFingerprint(tree)
				if err != nil {
					t.Fatal(err)
				}
				if !containsHistoricalDigest(catalog["auth"], digest) {
					t.Fatalf("official %s stamp %q is not recognized", source.Version, version)
				}
			}
			tree["SKILL.md"] = &fstest.MapFile{Data: append(append([]byte{}, original...), []byte("\nuser modification")...)}
			digest, err := treeFingerprint(tree)
			if err != nil {
				t.Fatal(err)
			}
			if containsHistoricalDigest(catalog["auth"], digest) {
				t.Fatal("modified official Skill was trusted")
			}
		})
	}
}

func containsHistoricalDigest(known []string, digest string) bool {
	for _, candidate := range known {
		if candidate == digest {
			return true
		}
	}
	return false
}
