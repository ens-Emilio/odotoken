package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/ens-Emilio/odotoken/internal/config"
)

// Store holds the client API keys in memory and persists changes to
// config.json. Keys are compared in constant time and only ever returned
// masked, so a dashboard page cannot leak a working credential.
type Store struct {
	mu     sync.RWMutex
	keys   map[string]config.ClientKeyConfig
	path   string
	logger *slog.Logger
}

// NewStore copies the configured keys. configPath is where Set/Delete persist;
// an empty path keeps changes in memory (tests, read-only setups).
func NewStore(keys map[string]config.ClientKeyConfig, configPath string, logger *slog.Logger) *Store {
	copied := make(map[string]config.ClientKeyConfig, len(keys))
	for k, v := range keys {
		if k != "" {
			copied[k] = v
		}
	}
	return &Store{keys: copied, path: configPath, logger: logger}
}

// Entry is one key as the API returns it: masked, never the secret.
type Entry struct {
	Key                 string  `json:"key"`
	Client              string  `json:"client"`
	Project             string  `json:"project"`
	DailyCostLimitUSD   float64 `json:"daily_cost_limit_usd"`
	MonthlyCostLimitUSD float64 `json:"monthly_cost_limit_usd"`
	DailyTokenLimit     int     `json:"daily_tokens_limit"`
	MonthlyTokenLimit   int     `json:"monthly_tokens_limit"`
}

// GuardConfig converts the API-facing entry to the guard's quota type.
func (e Entry) GuardConfig() config.GuardConfig {
	return config.GuardConfig{DailyCostLimitUSD: e.DailyCostLimitUSD, MonthlyCostLimitUSD: e.MonthlyCostLimitUSD,
		DailyTokenLimit: e.DailyTokenLimit, MonthlyTokenLimit: e.MonthlyTokenLimit}
}

// Mask keeps a stable, non-secret identifier: the first four characters and
// the last four, so two keys are distinguishable without being usable.
func Mask(key string) string {
	if len(key) <= 8 {
		return "…"
	}
	return key[:4] + "…" + key[len(key)-4:]
}

// Resolve finds the key config for a presented credential. A masked identifier
// is accepted too, which is what the dashboard sends back on update/delete.
func (s *Store) Resolve(presented string) (config.ClientKeyConfig, bool) {
	if presented == "" {
		return config.ClientKeyConfig{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for key, cfg := range s.keys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(presented)) == 1 {
			return cfg, true
		}
	}
	var match config.ClientKeyConfig
	found := 0
	for key, cfg := range s.keys {
		if Mask(key) == presented {
			match = cfg
			found++
		}
	}
	return match, found == 1
}

// List returns every key masked, ordered by client then project so the table
// does not reshuffle between polls.
func (s *Store) List() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Entry, 0, len(s.keys))
	for key, cfg := range s.keys {
		out = append(out, Entry{
			Key:                 Mask(key),
			Client:              cfg.Client,
			Project:             cfg.Project,
			DailyCostLimitUSD:   cfg.DailyCostLimitUSD,
			MonthlyCostLimitUSD: cfg.MonthlyCostLimitUSD,
			DailyTokenLimit:     cfg.DailyTokenLimit,
			MonthlyTokenLimit:   cfg.MonthlyTokenLimit,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Client != out[j].Client {
			return out[i].Client < out[j].Client
		}
		if out[i].Project != out[j].Project {
			return out[i].Project < out[j].Project
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// Set creates or updates a key. An empty identifier generates a new secret and
// returns it once; a masked identifier updates the existing record in place.
func (s *Store) Set(identifier string, cfg config.ClientKeyConfig) (plaintext string, entry Entry, err error) {
	if strings.TrimSpace(cfg.Client) == "" || strings.TrimSpace(cfg.Project) == "" {
		return "", Entry{}, ErrMissingIdentity
	}
	if cfg.DailyCostLimitUSD < 0 || cfg.MonthlyCostLimitUSD < 0 || cfg.DailyTokenLimit < 0 || cfg.MonthlyTokenLimit < 0 {
		return "", Entry{}, ErrNegativeLimit
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	target := ""
	switch {
	case identifier == "":
		secret, genErr := generateKey()
		if genErr != nil {
			return "", Entry{}, genErr
		}
		target = secret
		plaintext = secret
	default:
		for key := range s.keys {
			if subtle.ConstantTimeCompare([]byte(key), []byte(identifier)) == 1 {
				target = key
				break
			}
		}
		if target == "" {
			matches := 0
			for key := range s.keys {
				if Mask(key) == identifier {
					target = key
					matches++
				}
			}
			if matches != 1 {
				return "", Entry{}, ErrNotFound
			}
		}
	}

	next := make(map[string]config.ClientKeyConfig, len(s.keys)+1)
	for key, existing := range s.keys {
		next[key] = existing
	}
	next[target] = cfg
	if err := s.persist(next); err != nil {
		return "", Entry{}, err
	}
	s.keys = next
	return plaintext, Entry{
		Key:                 Mask(target),
		Client:              cfg.Client,
		Project:             cfg.Project,
		DailyCostLimitUSD:   cfg.DailyCostLimitUSD,
		MonthlyCostLimitUSD: cfg.MonthlyCostLimitUSD,
		DailyTokenLimit:     cfg.DailyTokenLimit,
		MonthlyTokenLimit:   cfg.MonthlyTokenLimit,
	}, nil
}

// Delete revokes a key by its masked or full form.
func (s *Store) Delete(identifier string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	target := ""
	for key := range s.keys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(identifier)) == 1 || Mask(key) == identifier {
			if target != "" {
				return ErrNotFound
			}
			target = key
		}
	}
	if target == "" {
		return ErrNotFound
	}
	next := make(map[string]config.ClientKeyConfig, len(s.keys))
	for key, cfg := range s.keys {
		if key != target {
			next[key] = cfg
		}
	}
	if err := s.persist(next); err != nil {
		return err
	}
	s.keys = next
	return nil
}

func (s *Store) persist(keys map[string]config.ClientKeyConfig) error {
	if s.path == "" {
		return nil
	}
	return config.PersistClientKeys(s.path, keys)
}

func generateKey() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return "odt_" + hex.EncodeToString(buf), nil
}
