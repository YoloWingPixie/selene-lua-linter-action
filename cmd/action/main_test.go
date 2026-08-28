package main

import (
	"path/filepath"
	"testing"
)

func TestLoadConfigRequiresConfigPath(t *testing.T) {
	t.Setenv("GITHUB_WORKSPACE", t.TempDir())
	t.Setenv("INPUT_CONFIG-PATH", "")

	if _, err := loadConfig(nil); err == nil {
		t.Fatal("loadConfig() succeeded without config-path")
	}
}

func TestLoadConfigResolvesPathsFromWorkspace(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("GITHUB_WORKSPACE", workspace)
	t.Setenv("INPUT_CONFIG-PATH", "selene.toml")
	t.Setenv("INPUT_WORKING-DIRECTORY", "project")

	config, err := loadConfig(nil)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if config.WorkingDirectory != filepath.Join(workspace, "project") {
		t.Fatalf("WorkingDirectory = %q", config.WorkingDirectory)
	}
	if config.ConfigPath != filepath.Join(workspace, "project", "selene.toml") {
		t.Fatalf("ConfigPath = %q", config.ConfigPath)
	}
}

func TestLoadConfigReadsArguments(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("GITHUB_WORKSPACE", workspace)

	config, err := loadConfig([]string{
		"--config-path", "selene.toml",
		"--working-directory", "project",
		"--fail-on-warnings", "true",
	})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if !config.FailOnWarnings {
		t.Fatal("FailOnWarnings = false")
	}
	if config.ConfigPath != filepath.Join(workspace, "project", "selene.toml") {
		t.Fatalf("ConfigPath = %q", config.ConfigPath)
	}
}

func TestLoadConfigRejectsInvalidBoolean(t *testing.T) {
	t.Setenv("GITHUB_WORKSPACE", t.TempDir())

	if _, err := loadConfig([]string{"--config-path", "selene.toml", "--fail-on-warnings", "sometimes"}); err == nil {
		t.Fatal("loadConfig() accepted an invalid boolean")
	}
}

func TestResolveSeleneRejectsUnknownVariant(t *testing.T) {
	if _, err := resolveSelene("unknown"); err == nil {
		t.Fatal("resolveSelene() accepted an unknown variant")
	}
}

func TestWorkflowEscaping(t *testing.T) {
	if value := workflowEscapeData("line%1\nline2"); value != "line%251%0Aline2" {
		t.Fatalf("workflowEscapeData() = %q", value)
	}
	if value := workflowEscapeProperty("src/a,b.lua:title"); value != "src/a%2Cb.lua%3Atitle" {
		t.Fatalf("workflowEscapeProperty() = %q", value)
	}
}
