package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type SecretsStore struct {
	dir string
}

type secretsFile struct {
	Secrets map[string]string `json:"secrets"`
}

func NewSecretsStore(dir string) *SecretsStore {
	return &SecretsStore{dir: dir}
}

func (s *SecretsStore) Path() string {
	return filepath.Join(s.dir, "secrets.json")
}

func (s *SecretsStore) Get(key string) (string, bool, error) {
	file, err := s.load()
	if err != nil {
		return "", false, err
	}
	value, ok := file.Secrets[key]
	return value, ok, nil
}

func (s *SecretsStore) Set(key, value string) error {
	file, err := s.load()
	if err != nil {
		return err
	}
	file.Secrets[key] = value
	return s.save(file)
}

func (s *SecretsStore) Delete(key string) error {
	file, err := s.load()
	if err != nil {
		return err
	}
	delete(file.Secrets, key)
	return s.save(file)
}

func (s *SecretsStore) load() (*secretsFile, error) {
	data, err := os.ReadFile(s.Path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &secretsFile{Secrets: map[string]string{}}, nil
		}
		return nil, fmt.Errorf("read secrets: %w", err)
	}

	var file secretsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode secrets: %w", err)
	}
	if file.Secrets == nil {
		file.Secrets = map[string]string{}
	}
	return &file, nil
}

func (s *SecretsStore) save(file *secretsFile) error {
	if file.Secrets == nil {
		file.Secrets = map[string]string{}
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create secrets dir: %w", err)
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode secrets: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(s.Path(), data, 0o600); err != nil {
		return fmt.Errorf("write secrets: %w", err)
	}
	return nil
}
