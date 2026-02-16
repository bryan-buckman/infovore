package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	EnvDemo = "demo"
	EnvProd = "prod"

	DemoBaseURL = "https://demo-api.kalshi.co"
	ProdBaseURL = "https://api.elections.kalshi.com"
)

type Config struct {
	APIKeyID       string
	PrivateKeyPath string // set by Load() for env-var/file mode
	PrivateKeyData []byte // set by LoadFromSettings() for DB mode
	Environment    string
	BaseURL        string
}

// Load loads Kalshi configuration from environment variables.
func Load() (*Config, error) {
	keyID := os.Getenv("KALSHI_API_KEY_ID")
	if keyID == "" {
		return nil, fmt.Errorf("KALSHI_API_KEY_ID environment variable is required")
	}

	keyPath := os.Getenv("KALSHI_PRIVATE_KEY_PATH")
	if keyPath == "" {
		return nil, fmt.Errorf("KALSHI_PRIVATE_KEY_PATH environment variable is required")
	}

	// Expand ~ to home directory
	if strings.HasPrefix(keyPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		keyPath = home + keyPath[1:]
	}

	// Verify key file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("private key file not found: %s", keyPath)
	}

	env := os.Getenv("KALSHI_ENV")
	if env == "" {
		env = EnvDemo
	}

	var baseURL string
	switch env {
	case EnvDemo:
		baseURL = DemoBaseURL
	case EnvProd:
		baseURL = ProdBaseURL
	default:
		return nil, fmt.Errorf("invalid KALSHI_ENV: %s (must be 'demo' or 'prod')", env)
	}

	return &Config{
		APIKeyID:       keyID,
		PrivateKeyPath: keyPath,
		Environment:    env,
		BaseURL:        baseURL,
	}, nil
}

// SettingsGetter is a minimal interface for reading settings from a database.
// This avoids importing the full database package.
type SettingsGetter interface {
	GetSetting(key string) (string, error)
}

// LoadFromSettings loads Kalshi configuration from the database settings table.
func LoadFromSettings(store SettingsGetter) (*Config, error) {
	keyID, _ := store.GetSetting("kalshi_api_key_id")
	if keyID == "" {
		return nil, fmt.Errorf("Kalshi API key not configured (set in Settings)")
	}

	privateKey, _ := store.GetSetting("kalshi_private_key")
	if privateKey == "" {
		return nil, fmt.Errorf("Kalshi private key not configured (set in Settings)")
	}

	env, _ := store.GetSetting("kalshi_environment")
	if env == "" {
		env = EnvProd
	}

	var baseURL string
	switch env {
	case EnvProd:
		baseURL = ProdBaseURL
	case EnvDemo:
		baseURL = DemoBaseURL
	default:
		return nil, fmt.Errorf("invalid kalshi_environment: %s", env)
	}

	return &Config{
		APIKeyID:       keyID,
		PrivateKeyData: []byte(privateKey),
		Environment:    env,
		BaseURL:        baseURL,
	}, nil
}
