// Package webhook dispatches outbound notifications (Discord, Slack, or a
// generic JSON POST) for events this fork cares about: a container crashing,
// a new image version being available, a disk-space warning, and a tracked
// package's delivery status changing. Config is a single shared JSON file
// under /etc/casaos/ that both this module and AppManagement read directly
// from disk - see the webhook notifications plan for why (AppManagement has
// no in-process access to this module's HTTP API, and vice versa, but both
// already read their own config straight off disk).
package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"go.uber.org/zap"
)

var ConfigFilePath = filepath.Join(constants.DefaultConfigPath, "webhooks.json")

// Version is the running fork release (e.g. "v1.8.7"), set once at startup
// by main.go. Stamped onto every outbound notification so a message on
// Discord/Slack/etc always shows which build sent it - otherwise
// confirming "was this notification from before or after the fix shipped"
// means SSHing in to check the version by hand.
var Version string

// DeviceName identifies which box sent a notification - the OS hostname,
// set once at startup by main.go. Prefixed onto every outbound
// notification's title (see payloadFor) so a disk-warning/etc alert is
// still identifiable at a glance on a phone's notification banner, which
// usually only shows the title - useful the moment there's more than one
// CasaOS box sharing the same webhook destination. Mirrors the identical
// field in AppManagement's own copy of this package.
var DeviceName string

type Destination struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Type   string   `json:"type"` // "discord" | "slack" | "generic"
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

type Config struct {
	DiskWarningThresholdPercent float64       `json:"diskWarningThresholdPercent"`
	Destinations                []Destination `json:"destinations"`
}

const defaultDiskWarningThresholdPercent = 90

// Load reads the shared webhook config from disk. A missing file isn't an
// error - it just means no webhooks are configured yet.
func Load() (*Config, error) {
	data, err := os.ReadFile(ConfigFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{DiskWarningThresholdPercent: defaultDiskWarningThresholdPercent}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.DiskWarningThresholdPercent == 0 {
		cfg.DiskWarningThresholdPercent = defaultDiskWarningThresholdPercent
	}
	return &cfg, nil
}

// Save writes the shared webhook config to disk.
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

var httpClient = &http.Client{Timeout: 5 * time.Second}

// imageUpdateEventGroup lets a destination subscribed to "image_update"
// (the only auto-update checkbox the settings UI has ever exposed) also
// receive "image_update_applied"/"image_update_failed" - two event types
// the auto-updater has always sent but that no UI checkbox could ever
// match, since they were only ever an exact-string match against
// dest.Events. Confirmed live: a destination correctly receiving
// container_crash notifications was receiving zero update notifications
// of any kind, including "update available", despite the checkbox being on.
var imageUpdateEventGroup = map[string]bool{
	"image_update":         true,
	"image_update_applied": true,
	"image_update_failed":  true,
}

func destinationWantsEvent(dest Destination, eventType string) bool {
	if containsString(dest.Events, eventType) {
		return true
	}
	return imageUpdateEventGroup[eventType] && containsString(dest.Events, "image_update")
}

// Send delivers a notification to every configured destination subscribed to
// eventType. Failures are logged, never returned - a broken webhook URL must
// never take down whatever background job triggered this.
func Send(eventType, title, message string, fields map[string]string) {
	cfg, err := Load()
	if err != nil {
		logger.Error("webhook: failed to load config", zap.Error(err))
		return
	}
	for _, dest := range cfg.Destinations {
		if !destinationWantsEvent(dest, eventType) {
			continue
		}
		go sendOne(dest, eventType, title, message, fields)
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func sendOne(dest Destination, eventType, title, message string, fields map[string]string) {
	body, err := payloadFor(dest.Type, eventType, title, message, fields)
	if err != nil {
		logger.Error("webhook: failed to build payload", zap.Error(err), zap.String("destination", dest.Name))
		return
	}
	req, err := http.NewRequest(http.MethodPost, dest.URL, bytes.NewReader(body))
	if err != nil {
		logger.Error("webhook: failed to build request", zap.Error(err), zap.String("destination", dest.Name))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := httpClient.Do(req)
	if err != nil {
		logger.Error("webhook: request failed", zap.Error(err), zap.String("destination", dest.Name))
		return
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		logger.Error("webhook: destination returned an error status", zap.Int("status", res.StatusCode), zap.String("destination", dest.Name))
	}
}

// SendTest sends a synchronous test notification to a single ad-hoc
// destination (type + URL, not necessarily saved yet), returning an error if
// delivery failed - used by the "send test" button in Settings before a
// destination is saved.
func SendTest(destType, destURL string) error {
	body, err := payloadFor(destType, "test", "Test notification", "If you can see this, your webhook is configured correctly.", nil)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, destURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("destination returned status %d", res.StatusCode)
	}
	return nil
}

func payloadFor(destType, eventType, title, message string, fields map[string]string) ([]byte, error) {
	if DeviceName != "" {
		title = "[" + DeviceName + "] " + title
	}
	switch destType {
	case "discord":
		embed := map[string]interface{}{
			"title":       title,
			"description": message,
			"color":       colorFor(eventType),
			"fields":      discordFields(fields),
		}
		if Version != "" {
			embed["footer"] = map[string]interface{}{"text": "casaos " + Version}
		}
		return json.Marshal(map[string]interface{}{
			"embeds": []map[string]interface{}{embed},
		})
	case "slack":
		text := "*" + title + "*\n" + message
		if Version != "" {
			text += "\n_casaos " + Version + "_"
		}
		return json.Marshal(map[string]interface{}{
			"text": text,
		})
	default:
		return json.Marshal(map[string]interface{}{
			"event":       eventType,
			"title":       title,
			"message":     message,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
			"forkVersion": Version,
			"device":      DeviceName,
			"data":        fields,
		})
	}
}

func discordFields(fields map[string]string) []map[string]interface{} {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]map[string]interface{}, 0, len(fields))
	for _, name := range names {
		result = append(result, map[string]interface{}{"name": name, "value": fields[name], "inline": true})
	}
	return result
}

func colorFor(eventType string) int {
	switch eventType {
	case "container_crash":
		return 0xE74C3C // red
	case "disk_warning":
		return 0xF39C12 // orange
	case "image_update":
		return 0x3498DB // blue
	case "package_update":
		return 0x2ECC71 // green
	default:
		return 0x7F8C8D // grey
	}
}
