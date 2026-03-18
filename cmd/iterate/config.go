package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type iterConfig struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	OllamaBaseURL string `json:"ollama_base_url,omitempty"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "config.json")
}

func loadConfig() iterConfig {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return iterConfig{}
	}
	var cfg iterConfig
	json.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(cfg iterConfig) {
	path := configPath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0o644)
}
