package autoupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
)

var NotifiedFilePath = filepath.Join(constants.DefaultConfigPath, "autoupdate-notified.json")

// NotifiedTracker tracks which (app, service, tag) update notifications
// have already been sent, persisted to disk so a service restart doesn't
// cause the same still-pending update to be re-notified. Previously this
// lived only in an in-memory map that reset on every restart of
// casaos-app-management - which happens on every update.sh run, not just
// occasionally - so any update still pending got re-announced every time.
type NotifiedTracker struct {
	mu   sync.Mutex
	keys map[string]bool
}

// LoadNotifiedTracker reads the persisted set of already-notified keys. A
// missing file isn't an error - it just means nothing has been notified yet.
func LoadNotifiedTracker() (*NotifiedTracker, error) {
	t := &NotifiedTracker{keys: map[string]bool{}}

	data, err := os.ReadFile(NotifiedFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return t, nil
		}
		return nil, err
	}

	var keys map[string]bool
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, err
	}
	if keys != nil {
		t.keys = keys
	}
	return t, nil
}

// AlreadyNotified reports whether key was already notified.
func (t *NotifiedTracker) AlreadyNotified(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.keys[key]
}

// MarkNotified records key as notified and persists immediately, so the
// state survives even if the process restarts right after.
func (t *NotifiedTracker) MarkNotified(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.keys[key] {
		return
	}
	if t.keys == nil {
		t.keys = map[string]bool{}
	}
	t.keys[key] = true
	if err := t.save(); err != nil {
		// best-effort - worst case a crash right after this notification
		// causes one re-notification on next start, same as before this
		// tracker existed at all, not a regression.
		delete(t.keys, key)
	}
}

func (t *NotifiedTracker) save() error {
	data, err := json.MarshalIndent(t.keys, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(NotifiedFilePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(NotifiedFilePath, data, 0o600)
}
