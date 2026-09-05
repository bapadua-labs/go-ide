package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const maxRecentFolders = 10

type ideConfig struct {
	LastFolder    string   `json:"last_folder"`
	RecentFolders []string `json:"recent_folders"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "go-ide", "state.json"), nil
}

func loadConfig() ideConfig {
	path, err := configPath()
	if err != nil {
		return ideConfig{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ideConfig{}
	}

	var cfg ideConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ideConfig{}
	}
	cfg.RecentFolders = pruneExistingFolders(cfg.RecentFolders)
	if cfg.LastFolder != "" && !folderExists(cfg.LastFolder) {
		cfg.LastFolder = ""
	}
	return cfg
}

func (cfg *ideConfig) save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (cfg *ideConfig) setLastFolder(path string) {
	cfg.LastFolder = path
}

func (cfg *ideConfig) addRecentFolder(path string) {
	path = filepath.Clean(path)
	var filtered []string
	for _, p := range cfg.RecentFolders {
		if p != path {
			filtered = append(filtered, p)
		}
	}
	cfg.RecentFolders = append([]string{path}, filtered...)
	if len(cfg.RecentFolders) > maxRecentFolders {
		cfg.RecentFolders = cfg.RecentFolders[:maxRecentFolders]
	}
}

func folderExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func pruneExistingFolders(paths []string) []string {
	var out []string
	for _, p := range paths {
		if folderExists(p) {
			out = append(out, p)
		}
	}
	return out
}
