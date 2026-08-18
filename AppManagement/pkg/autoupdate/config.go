// Package autoupdate persists a per-app auto-update policy under
// /etc/casaos/autoupdate.json - AppManagement-only, unlike pkg/webhook's
// shared config, since compose/container management is entirely this
// module's domain and the root module never touches it.
package autoupdate

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
)

var ConfigFilePath = filepath.Join(constants.DefaultConfigPath, "autoupdate.json")

// AppSettings holds two independent choices per app: whether to actually
// pull+recreate on a newer tag, and whether to fire a webhook when one is
// found. They're independent on purpose - e.g. an app that's fine to
// notify about but too risky to touch unattended, or one where you've
// already seen the notification enough times and just want it silently
// auto-updated from now on.
type AppSettings struct {
	AutoUpdate bool `json:"autoUpdate"`
	Notify     bool `json:"notify"`
}

// defaultSettings is what an app with no explicit entry gets: notify but
// never auto-update. The safe default - nothing changes unattended until a
// user explicitly checks the Auto-Update box for that app.
var defaultSettings = AppSettings{AutoUpdate: false, Notify: true}

type Config struct {
	// Apps is keyed by the same app-name string used everywhere else in
	// this codebase (ComposeApp.Name / MyAppList.Name). Apps with no entry
	// here get defaultSettings - see SettingsFor.
	Apps map[string]AppSettings `json:"apps"`
}

// Load reads the shared auto-update config from disk. A missing file isn't
// an error - it just means every app is still on the default settings.
func Load() (*Config, error) {
	data, err := os.ReadFile(ConfigFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Apps: map[string]AppSettings{}}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Apps == nil {
		cfg.Apps = map[string]AppSettings{}
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

// SettingsFor returns the configured settings for appName, or
// defaultSettings (notify, don't auto-update) if the app has no explicit
// entry.
func SettingsFor(cfg *Config, appName string) AppSettings {
	if s, ok := cfg.Apps[appName]; ok {
		return s
	}
	return defaultSettings
}
