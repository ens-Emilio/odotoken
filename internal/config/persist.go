package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// PersistClientKeys writes server.client_keys into the config file, leaving
// every other field byte-identical in meaning. The dashboard and the CLI both
// write here, so the file stays the single source of truth.
func PersistClientKeys(path string, keys map[string]ClientKeyConfig) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	root := map[string]json.RawMessage{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}
	server := map[string]json.RawMessage{}
	if raw := root["server"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &server); err != nil {
			return fmt.Errorf("parse server block: %w", err)
		}
	}
	body, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	server["client_keys"] = body
	serverBody, err := json.Marshal(server)
	if err != nil {
		return err
	}
	root["server"] = serverBody
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(out, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ClientKeysFromConfig returns the configured client keys, never nil.
func (c Config) ClientKeysFromConfig() map[string]ClientKeyConfig {
	if c.Server.ClientKeys == nil {
		return map[string]ClientKeyConfig{}
	}
	return c.Server.ClientKeys
}
