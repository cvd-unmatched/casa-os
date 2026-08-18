// Package autoupdate persists a per-app auto-update policy (auto/notify/off)
// under /etc/casaos/autoupdate.json - AppManagement-only, unlike
// pkg/webhook's shared config, since compose/container management is
// entirely this module's domain and the root module never touches it.
package autoupdate

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
)

var ConfigFilePath = filepath.Join(constants.DefaultConfigPath, "autoupdate.json")

type Policy string

const (
	PolicyAuto   Policy = "auto"
	PolicyNotify Policy = "notify" // default for any app with no entry below
	PolicyOff    Policy = "off"
)

type Config struct {
	// AppPolicies is keyed by the same app-name string used everywhere else
	// in this codebase (ComposeApp.Name / MyAppList.Name). Apps with no
	// entry here are treated as PolicyNotify - see PolicyFor.
	AppPolicies map[string]Policy `json:"appPolicies"`
}

// Load reads the shared auto-update config from disk. A missing file isn't
// an error - it just means every app is still on the default policy.
func Load() (*Config, error) {
	data, err := os.ReadFile(ConfigFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{AppPolicies: map[string]Policy{}}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.AppPolicies == nil {
		cfg.AppPolicies = map[string]Policy{}
	}
	return &cfg, nil
}

// Save writes the auto-update config to disk.
func Save(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ConfigFilePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(ConfigFilePath, data, 0o600)
}

// PolicyFor returns the configured policy for appName, or PolicyNotify (the
// safe default) if the app has no explicit entry - nothing auto-updates
// until a user opts an app in.
func PolicyFor(cfg *Config, appName string) Policy {
	if p, ok := cfg.AppPolicies[appName]; ok && p != "" {
		return p
	}
	return PolicyNotify
}
