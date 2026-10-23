package config

import (
	"encoding/json"
	"os"
)

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		TokenCounter: TokenCounterConfig{
			Enabled: true,
			Heuristic: HeuristicConfig{
				CharsPerToken: 4.0,
				WordsPerToken: 0.75,
			},
			Providers: make(map[string]ProviderConfig),
		},
		Sessions: SessionsConfig{
			Enabled: true,
			Paths:   make(map[string]string),
		},
		Guard: GuardConfig{
			WarningThresholdPct: 80.0,
		},
	}
}

// Load reads config from a JSON file, falling back to defaults.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		path = "config.json"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	applyEnvOverrides(&cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	for name, prov := range cfg.TokenCounter.Providers {
		if prov.APIKeyEnv != "" {
			if key := os.Getenv(prov.APIKeyEnv); key != "" {
				prov.Enabled = true
				cfg.TokenCounter.Providers[name] = prov
			}
		}
	}
}