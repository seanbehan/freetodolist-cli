package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// credentials persists per-base-URL login state. We bucket by baseURL so the
// same machine can simultaneously be logged into a local dev server and prod.
type credentials struct {
	BaseURL     string `json:"base_url"`
	AccessToken string `json:"access_token"`
	ClientID    string `json:"client_id,omitempty"`
}

type credentialsFile struct {
	// Profiles keyed by base URL (e.g. "https://freetodolist.com",
	// "http://localhost:3100").
	Profiles map[string]credentials `json:"profiles"`
}

func credentialsPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

// configDir resolves to $XDG_CONFIG_HOME/freetodolist or ~/.config/freetodolist
// (created on demand with 0700 perms).
func configDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	dir := filepath.Join(base, "freetodolist")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func loadCredentialsFile() (*credentialsFile, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &credentialsFile{Profiles: map[string]credentials{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var f credentialsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse credentials file %s: %w", path, err)
	}
	if f.Profiles == nil {
		f.Profiles = map[string]credentials{}
	}
	return &f, nil
}

func saveCredentialsFile(f *credentialsFile) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	// Write to a tempfile and rename so a crash mid-write doesn't truncate.
	tmp, err := os.CreateTemp(filepath.Dir(path), "credentials-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

// loadCredentialsFor returns the saved credentials for baseURL, or nil if
// none. Callers should treat absence as "not logged in".
func loadCredentialsFor(baseURL string) (*credentials, error) {
	f, err := loadCredentialsFile()
	if err != nil {
		return nil, err
	}
	c, ok := f.Profiles[baseURL]
	if !ok {
		return nil, nil
	}
	return &c, nil
}

func saveCredentialsFor(c credentials) error {
	f, err := loadCredentialsFile()
	if err != nil {
		return err
	}
	f.Profiles[c.BaseURL] = c
	return saveCredentialsFile(f)
}

func deleteCredentialsFor(baseURL string) (bool, error) {
	f, err := loadCredentialsFile()
	if err != nil {
		return false, err
	}
	if _, ok := f.Profiles[baseURL]; !ok {
		return false, nil
	}
	delete(f.Profiles, baseURL)
	return true, saveCredentialsFile(f)
}
