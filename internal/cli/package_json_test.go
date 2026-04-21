package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPackageJSONIncludesSkillsInNPMPackage(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("..", "..", "package.json"))
	if err != nil {
		t.Fatalf("ReadFile(package.json) error = %v", err)
	}

	var manifest struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatalf("Unmarshal(package.json) error = %v", err)
	}

	requiredPatterns := []string{
		"skills/**/SKILL.md",
		"skills/**/agents/**",
		"skills/**/references/**",
	}
	for _, required := range requiredPatterns {
		if !containsString(manifest.Files, required) {
			t.Fatalf("package.json files must include %s, got %v", required, manifest.Files)
		}
	}
}

func TestPackageJSONPublishingMetadata(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("..", "..", "package.json"))
	if err != nil {
		t.Fatalf("ReadFile(package.json) error = %v", err)
	}

	var manifest struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Config  struct {
			BinaryName              string `json:"binaryName"`
			DownloadBaseURLTemplate string `json:"downloadBaseURLTemplate"`
		} `json:"config"`
		PublishConfig struct {
			Registry string `json:"registry"`
			Access   string `json:"access"`
		} `json:"publishConfig"`
		Repository struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatalf("Unmarshal(package.json) error = %v", err)
	}

	if manifest.Name != "@qfeius/contract-cli" {
		t.Fatalf("package name = %q, want @qfeius/contract-cli", manifest.Name)
	}
	if manifest.Version != "0.1.0-beta.1" {
		t.Fatalf("package version = %q, want 0.1.0-beta.1", manifest.Version)
	}
	if manifest.PublishConfig.Registry != "https://registry.npmjs.org/" {
		t.Fatalf("publish registry = %q, want https://registry.npmjs.org/", manifest.PublishConfig.Registry)
	}
	if manifest.PublishConfig.Access != "public" {
		t.Fatalf("publish access = %q, want public", manifest.PublishConfig.Access)
	}
	if manifest.Config.BinaryName != "contract-cli" {
		t.Fatalf("binary name = %q, want contract-cli", manifest.Config.BinaryName)
	}
	const wantDownloadBaseURL = "https://github.com/qfeius/contract-cli/releases/download/v{version}"
	if manifest.Config.DownloadBaseURLTemplate != wantDownloadBaseURL {
		t.Fatalf("download base URL template = %q, want %q", manifest.Config.DownloadBaseURLTemplate, wantDownloadBaseURL)
	}
	if manifest.Repository.Type != "git" {
		t.Fatalf("repository type = %q, want git", manifest.Repository.Type)
	}
	const wantRepositoryURL = "git+https://github.com/qfeius/contract-cli.git"
	if manifest.Repository.URL != wantRepositoryURL {
		t.Fatalf("repository url = %q, want %q", manifest.Repository.URL, wantRepositoryURL)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
