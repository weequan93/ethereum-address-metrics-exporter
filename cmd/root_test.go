package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigResolvesRelativeStateFileFromConfigDir(t *testing.T) {
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "config.yaml")

	err := os.WriteFile(configFile, []byte(`global:
  stateFile: ./data/exporter-state.json
`), 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := filepath.Join(configDir, "data", "exporter-state.json")
	if cfg.GlobalConfig.StateFile != want {
		t.Fatalf("state file = %q, want %q", cfg.GlobalConfig.StateFile, want)
	}
}

func TestLoadConfigKeepsAbsoluteStateFile(t *testing.T) {
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "config.yaml")
	stateFile := filepath.Join(t.TempDir(), "exporter-state.json")

	err := os.WriteFile(configFile, []byte("global:\n  stateFile: "+stateFile+"\n"), 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.GlobalConfig.StateFile != stateFile {
		t.Fatalf("state file = %q, want %q", cfg.GlobalConfig.StateFile, stateFile)
	}
}
