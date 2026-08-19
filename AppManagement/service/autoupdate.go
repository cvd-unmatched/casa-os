package service

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/CasaOS-AppManagement/common"
	"github.com/IceWhaleTech/CasaOS-AppManagement/model"
	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/autoupdate"
	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/docker"
	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/webhook"
	v1 "github.com/IceWhaleTech/CasaOS-AppManagement/service/v1"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/Masterminds/semver/v3"
	"go.uber.org/zap"
)

// AutoUpdateAppStatus is what GET /v1/autoupdate/apps serves - one row per
// app (not per service - a multi-service compose app is summarized by
// whichever service has an update available, or its first service if none
// do, since the UI's controls are per-app, not per-service).
type AutoUpdateAppStatus struct {
	// Name is the internal identifier (compose project name / container
	// name) - what policy lookups and the /apps/:name/* API are keyed on.
	// For an app installed without an explicit `name:` in its compose file,
	// this is a Docker-style random string (e.g. "friendly_jeanie") - same
	// one the dashboard itself falls back to for that app, not something
	// unique to this feature.
	Name            string `json:"name"`
	DisplayName     string `json:"displayName"` // the dashboard's app title - same as Name when no nicer title is stored
	AppType         string `json:"appType"`     // "v1" | "v2"
	CurrentImage    string `json:"currentImage"`
	CurrentTag      string `json:"currentTag"`
	LatestKnownTag  string `json:"latestKnownTag"`
	UpdateAvailable bool   `json:"updateAvailable"`
	AutoUpdate      bool   `json:"autoUpdate"`
	Notify          bool   `json:"notify"`
	IsUncontrolled  bool   `json:"isUncontrolled"`
}

var (
	autoUpdateStatusMu    sync.Mutex
	autoUpdateStatusCache []AutoUpdateAppStatus

	notifiedTrackerOnce sync.Once
	notifiedTracker     *autoupdate.NotifiedTracker
)

// sharedNotifiedTracker is loaded once (from disk, if present) and reused
// by both the cron and manual "recheck" clicks, so neither path can
// re-announce an update the other already notified about.
func sharedNotifiedTracker() *autoupdate.NotifiedTracker {
	notifiedTrackerOnce.Do(func() {
		t, err := autoupdate.LoadNotifiedTracker()
		if err != nil {
			logger.Error("autoupdate: failed to load notified-state, starting empty", zap.Error(err))
			t, _ = autoupdate.LoadNotifiedTracker() // retry once against a clean read; falls through to empty below on repeat failure
		}
		if t == nil {
			t = &autoupdate.NotifiedTracker{}
		}
		notifiedTracker = t
	})
	return notifiedTracker
}

// GetAutoUpdateStatus reconciles the cached per-app status (populated by
// the cron tick - see CheckAndApplyAutoUpdates) against the CURRENT live
// app list, rather than just serving whatever's cached: an app installed
// since the last cron tick would otherwise be invisible for up to an hour,
// and an uninstalled app would otherwise linger in the response forever.
// Doesn't hit any registry itself - a newly installed app gets a cheap,
// local-only placeholder row (current image/tag, no latest-tag info yet)
// until the next cron tick actually checks it.
func GetAutoUpdateStatus(ctx context.Context) []AutoUpdateAppStatus {
	cached := map[string]AutoUpdateAppStatus{}
	autoUpdateStatusMu.Lock()
	for _, s := range autoUpdateStatusCache {
		cached[s.AppType+":"+s.Name] = s
	}
	autoUpdateStatusMu.Unlock()

	cfg, err := autoupdate.Load()
	if err != nil {
		cfg = &autoupdate.Config{Apps: map[string]autoupdate.AppSettings{}}
	}

	var out []AutoUpdateAppStatus

	composeApps, err := MyService.Compose().List(ctx)
	if err != nil {
		logger.Error("autoupdate: failed to list compose apps for status", zap.Error(err))
	}
	for _, app := range composeApps {
		if s, ok := cached["v2:"+app.Name]; ok {
			out = append(out, s)
			continue
		}
		out = append(out, placeholderComposeAppStatus(app, cfg))
	}

	casaOSApps, _ := MyService.Docker().GetContainerAppList(nil, nil, nil)
	if casaOSApps != nil {
		for _, app := range *casaOSApps {
			if s, ok := cached["v1:"+app.Name]; ok {
				out = append(out, s)
				continue
			}
			out = append(out, placeholderStandaloneAppStatus(app, cfg))
		}
	}

	return out
}

func placeholderComposeAppStatus(app *ComposeApp, cfg *autoupdate.Config) AutoUpdateAppStatus {
	uncontrolled := isComposeAppUncontrolled(app)
	settings := autoupdate.SettingsFor(cfg, app.Name)
	row := AutoUpdateAppStatus{
		Name:           app.Name,
		DisplayName:    composeAppDisplayName(app),
		AppType:        "v2",
		AutoUpdate:     settings.AutoUpdate,
		Notify:         settings.Notify,
		IsUncontrolled: uncontrolled,
	}
	if image := composeAppMainServiceImage(app); image != "" {
		row.CurrentImage = image
		_, row.CurrentTag = docker.ExtractImageAndTag(image)
	}
	return row
}

func placeholderStandaloneAppStatus(app model.MyAppList, cfg *autoupdate.Config) AutoUpdateAppStatus {
	_, currentTag := docker.ExtractImageAndTag(app.Image)
	settings := autoupdate.SettingsFor(cfg, app.Name)
	return AutoUpdateAppStatus{
		Name:           app.Name,
		DisplayName:    app.Name,
		AppType:        "v1",
		CurrentImage:   app.Image,
		CurrentTag:     currentTag,
		AutoUpdate:     settings.AutoUpdate,
		Notify:         settings.Notify,
		IsUncontrolled: app.IsUncontrolled,
	}
}

func updateAutoUpdateStatusCacheEntry(entry AutoUpdateAppStatus) {
	autoUpdateStatusMu.Lock()
	defer autoUpdateStatusMu.Unlock()
	for i, s := range autoUpdateStatusCache {
		if s.Name == entry.Name && s.AppType == entry.AppType {
			autoUpdateStatusCache[i] = entry
			return
		}
	}
	autoUpdateStatusCache = append(autoUpdateStatusCache, entry)
}

// previousAutoUpdateStatus returns the last cached status for (appType,
// name), if any - used by carryForwardKnownUpdate below.
func previousAutoUpdateStatus(appType, name string) (AutoUpdateAppStatus, bool) {
	autoUpdateStatusMu.Lock()
	defer autoUpdateStatusMu.Unlock()
	for _, s := range autoUpdateStatusCache {
		if s.AppType == appType && s.Name == name {
			return s, true
		}
	}
	return AutoUpdateAppStatus{}, false
}

// carryForwardKnownUpdate preserves a previously-detected available update
// when this round's check came back empty-handed (a registry error, a rate
// limit, or genuinely zero comparable tags this time) but the running app
// hasn't changed since - whatever newer tag was found before is presumably
// still out there, so a transient check hiccup shouldn't blank the panel
// back to "no comparable version tags found" and lose that. Confirmed live
// against ghcr.io/cvd-unmatched/movies flapping between "update available"
// and "no comparable" across consecutive hourly ticks with nothing actually
// changing registry-side. Never affects whether an update gets applied -
// newImageByService (checkComposeAppForUpdates/checkStandaloneAppForUpdates
// callers) only gets populated by a fresh, this-round confirmation.
func carryForwardKnownUpdate(row AutoUpdateAppStatus) AutoUpdateAppStatus {
	if row.UpdateAvailable {
		return row
	}
	prev, ok := previousAutoUpdateStatus(row.AppType, row.Name)
	if !ok || !prev.UpdateAvailable || prev.CurrentTag != row.CurrentTag {
		return row
	}
	row.LatestKnownTag = prev.LatestKnownTag
	row.UpdateAvailable = true
	return row
}

// pacingDelay is the minimum gap between two registry round-trips within
// one CheckAndApplyAutoUpdates run - a hard requirement, not a tunable
// optimization: the user's stack includes 15+ unauthenticated Docker Hub
// images sharing the host's IP-based ~100/6h manifest-pull rate limit, and
// checking rapidly/in parallel could exhaust that in one tick and start
// failing unrelated pulls for hours.
const pacingDelay = 2500 * time.Millisecond

// CheckAndApplyAutoUpdates walks every CasaOS-managed app - v2 compose
// apps via Compose().List, and v1 standalone apps via
// Docker().GetContainerAppList's casaOSApps return value only (never its
// localApps return value, which are unmanaged host containers CasaOS has
// no business touching) - checks each app's images against the registry
// for a newer semver tag, and for policy=auto apps applies the update.
// Called once per cron tick from main.go in place of the old,
// catalog-only checkImageUpdates.
func CheckAndApplyAutoUpdates(ctx context.Context) {
	cfg, err := autoupdate.Load()
	if err != nil {
		logger.Error("autoupdate: failed to load config", zap.Error(err))
		return
	}

	// updated per-app as the run progresses, not just once at the end - a
	// full pass across every app at pacingDelay apart can take a couple of
	// minutes, and the panel should show what's known so far rather than
	// nothing until the entire run completes.
	composeApps, err := MyService.Compose().List(ctx)
	if err != nil {
		logger.Error("autoupdate: failed to list compose apps", zap.Error(err))
	}
	for _, app := range composeApps {
		safeCheckAndMaybeApplyComposeApp(ctx, app, cfg)
		time.Sleep(pacingDelay)
	}

	casaOSApps, _ := MyService.Docker().GetContainerAppList(nil, nil, nil)
	if casaOSApps != nil {
		for _, app := range *casaOSApps {
			safeCheckAndMaybeApplyStandaloneApp(ctx, app, cfg)
			time.Sleep(pacingDelay)
		}
	}
}

// safeCheckAndMaybeApplyComposeApp recovers from a panic checking/applying
// one app so it can't abort the rest of the run - main.go's runAutoUpdateCheck
// already guards the whole cron tick, but without a per-app guard too, app
// #5 out of 40 panicking would leave apps #6-40 unchecked for that entire
// cycle.
func safeCheckAndMaybeApplyComposeApp(ctx context.Context, app *ComposeApp, cfg *autoupdate.Config) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("autoupdate: recovered from panic checking compose app",
				zap.String("app", app.Name), zap.Any("panic", r), zap.String("stack", string(debug.Stack())))
		}
	}()
	updateAutoUpdateStatusCacheEntry(checkAndMaybeApplyComposeApp(ctx, app, cfg))
}

func safeCheckAndMaybeApplyStandaloneApp(ctx context.Context, app model.MyAppList, cfg *autoupdate.Config) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("autoupdate: recovered from panic checking standalone app",
				zap.String("app", app.Name), zap.Any("panic", r), zap.String("stack", string(debug.Stack())))
		}
	}()
	updateAutoUpdateStatusCacheEntry(checkAndMaybeApplyStandaloneApp(ctx, app, cfg))
}

// RecheckApp forces one synchronous, read-only registry check for a single
// app (bypassing the cache) - never applies an update itself even under
// policy=auto, so a status-refresh click can't accidentally trigger a
// container recreate.
func RecheckApp(ctx context.Context, appName string) (AutoUpdateAppStatus, error) {
	cfg, err := autoupdate.Load()
	if err != nil {
		return AutoUpdateAppStatus{}, err
	}

	composeApps, err := MyService.Compose().List(ctx)
	if err == nil {
		if app, ok := composeApps[appName]; ok {
			row, _ := checkComposeAppForUpdates(ctx, app, cfg)
			updateAutoUpdateStatusCacheEntry(row)
			return row, nil
		}
	}

	casaOSApps, _ := MyService.Docker().GetContainerAppList(nil, nil, nil)
	if casaOSApps != nil {
		for _, app := range *casaOSApps {
			if app.Name == appName {
				row := checkStandaloneAppForUpdates(ctx, app, cfg)
				updateAutoUpdateStatusCacheEntry(row)
				return row, nil
			}
		}
	}

	return AutoUpdateAppStatus{}, fmt.Errorf("app %q not found among managed apps", appName)
}

func checkAndMaybeApplyComposeApp(ctx context.Context, app *ComposeApp, cfg *autoupdate.Config) AutoUpdateAppStatus {
	row, newImageByService := checkComposeAppForUpdates(ctx, app, cfg)

	if len(newImageByService) > 0 && row.AutoUpdate {
		firstAttempt := firstAutoUpdateAttempt(app.Name, row.LatestKnownTag)
		var tracked *webhook.TrackedMessage
		if firstAttempt {
			tracked = webhook.SendTrackable("image_update_applied", "Updating",
				fmt.Sprintf("Auto-updating %s to %s", row.DisplayName, row.LatestKnownTag),
				map[string]string{"app": app.Name, "tag": row.LatestKnownTag})
		}

		if err := app.UpdateImages(ctx, newImageByService); err != nil {
			logger.Error("autoupdate: failed to apply compose app update", zap.Error(err), zap.String("app", app.Name))
			if firstAttempt {
				webhook.TryEdit(tracked, "image_update_failed", "Update failed",
					fmt.Sprintf("Failed to auto-update %s: %s", row.DisplayName, err.Error()),
					map[string]string{"app": app.Name})
			}
		} else {
			// edits the "Updating" message in place when there is one
			// (the common case) instead of posting a second message -
			// falls back to a fresh post on a later retry that never sent
			// its own "Updating" this round (see firstAutoUpdateAttempt).
			webhook.TryEdit(tracked, "image_update_applied", "Updated",
				fmt.Sprintf("%s was auto-updated to %s", row.DisplayName, row.LatestKnownTag),
				map[string]string{"app": app.Name, "tag": row.LatestKnownTag})
		}
	}

	return row
}

func checkComposeAppForUpdates(ctx context.Context, app *ComposeApp, cfg *autoupdate.Config) (AutoUpdateAppStatus, map[string]string) {
	uncontrolled := isComposeAppUncontrolled(app)
	settings := autoupdate.SettingsFor(cfg, app.Name)

	row := AutoUpdateAppStatus{
		Name:           app.Name,
		DisplayName:    composeAppDisplayName(app),
		AppType:        "v2",
		AutoUpdate:     settings.AutoUpdate,
		Notify:         settings.Notify,
		IsUncontrolled: uncontrolled,
	}
	newImageByService := map[string]string{}
	anyCheckSucceeded := false

	for _, svc := range app.Services {
		img, currentTag := docker.ExtractImageAndTag(svc.Image)

		if row.CurrentImage == "" {
			row.CurrentImage = svc.Image
			row.CurrentTag = currentTag
		}

		// IsUncontrolled (above) still reflects CasaOS's own App Store
		// tag-alignment flag for display purposes, but this fork's
		// semver-based auto-update deliberately manages these apps anyway -
		// a different, tag-driven axis of "controlled" than the store's own
		// catalog-alignment check.
		newTag, ok := checkNewestTag(ctx, svc.Image, currentTag)
		if !ok {
			continue
		}
		anyCheckSucceeded = true

		if newTag != currentTag {
			// prefer surfacing a service that actually has an update in
			// the summary row, overwriting the "first service" default
			row.CurrentImage = svc.Image
			row.CurrentTag = currentTag
			row.LatestKnownTag = newTag
			row.UpdateAvailable = true
			newImageByService[svc.Name] = img + ":" + newTag
			// an auto-update app is about to update itself on this same
			// pass (see checkAndMaybeApplyComposeApp below) - "available"
			// would just be a redundant ping seconds before "updated".
			if settings.Notify && !settings.AutoUpdate {
				notifyImageUpdate(app.Name, row.DisplayName, svc.Name, newTag)
			}
		} else if row.LatestKnownTag == "" {
			row.LatestKnownTag = newTag
		}
	}

	if !anyCheckSucceeded {
		row = carryForwardKnownUpdate(row)
	}

	return row, newImageByService
}

// composeAppDisplayName picks the most meaningful name available for a
// compose app, in order:
//  1. A real stored title (WebAppGridItemAdapterV2's same StoreInfo().Title
//     the dashboard uses) - but only if it's not just app.Name copied
//     verbatim, which is what SetTitle falls back to at install time for
//     any compose file with no explicit top-level `name:` (the common case
//     for GitHub-imported apps - confirmed none of this user's apps set
//     that field, so this stored "title" is normally just the same random
//     Docker-style project name, not an improvement).
//  2. The main service's image repository name (e.g. "inventory" from
//     ghcr.io/cvd-unmatched/inventory:1.6.0) - almost always more
//     meaningful than a random project name, since it's literally the
//     thing being run.
//  3. The raw project name, as a last resort.
func composeAppDisplayName(app *ComposeApp) string {
	if storeInfo, err := app.StoreInfo(false); err == nil && storeInfo != nil {
		if title, ok := storeInfo.Title[common.DefaultLanguage]; ok && title != "" && title != app.Name {
			return title
		}
	}
	if image := composeAppMainServiceImage(app); image != "" {
		return imageDisplayName(image)
	}
	return app.Name
}

// composeAppMainServiceImage returns the image of whichever service is
// named "main_app" (the convention this user's installs consistently use -
// e.g. container names like "imaginative_okem-main_app-1"), or the first
// declared service if there's no such convention, or "" for an app with no
// services at all.
func composeAppMainServiceImage(app *ComposeApp) string {
	for _, svc := range app.Services {
		if svc.Name == "main_app" {
			return svc.Image
		}
	}
	if len(app.Services) > 0 {
		return app.Services[0].Image
	}
	return ""
}

// imageDisplayName extracts the last path segment of an image reference -
// "ghcr.io/cvd-unmatched/inventory:1.6.0" -> "inventory",
// "linuxserver/mariadb:11.4.5" -> "mariadb", "postgres:16-alpine" -> "postgres".
func imageDisplayName(image string) string {
	img, _ := docker.ExtractImageAndTag(image)
	parts := strings.Split(img, "/")
	return parts[len(parts)-1]
}

func isComposeAppUncontrolled(app *ComposeApp) bool {
	storeInfo, err := app.StoreInfo(false)
	if err != nil || storeInfo == nil {
		// apps with no x-casaos store extension (e.g. installed via the
		// GitHub import flow) always error here - that's normal, not a
		// reason to treat them as uncontrolled.
		return false
	}
	return storeInfo.IsUncontrolled != nil && *storeInfo.IsUncontrolled
}

func checkAndMaybeApplyStandaloneApp(ctx context.Context, app model.MyAppList, cfg *autoupdate.Config) AutoUpdateAppStatus {
	row := checkStandaloneAppForUpdates(ctx, app, cfg)

	if !row.UpdateAvailable || !row.AutoUpdate {
		return row
	}

	firstAttempt := firstAutoUpdateAttempt(app.Name, row.LatestKnownTag)
	var tracked *webhook.TrackedMessage
	if firstAttempt {
		tracked = webhook.SendTrackable("image_update_applied", "Updating",
			fmt.Sprintf("Auto-updating %s to %s", app.Name, row.LatestKnownTag),
			map[string]string{"app": app.Name, "tag": row.LatestKnownTag})
	}

	img, _ := docker.ExtractImageAndTag(app.Image)
	if err := applyStandaloneAppUpdate(ctx, app.ID, img+":"+row.LatestKnownTag); err != nil {
		logger.Error("autoupdate: failed to apply standalone app update", zap.Error(err), zap.String("app", app.Name))
		if firstAttempt {
			webhook.TryEdit(tracked, "image_update_failed", "Update failed",
				fmt.Sprintf("Failed to auto-update %s: %s", app.Name, err.Error()),
				map[string]string{"app": app.Name})
		}
	} else {
		// unlike the compose path, Install() only synchronously validates
		// and writes the new compose file before returning - the actual
		// pull+start happens in its own background goroutine (same
		// fire-and-forget shape the manual "Rebuild" action already has
		// today). This "Updated" notification reflects that the promotion
		// to a compose app was accepted, not a fully confirmed running
		// container - a real failure past this point would still surface
		// via CasaOS's own EventTypeAppInstallError message-bus event.
		//
		// Edits the "Updating" message in place when there is one (see the
		// matching comment in checkAndMaybeApplyComposeApp).
		webhook.TryEdit(tracked, "image_update_applied", "Updated",
			fmt.Sprintf("%s was auto-updated to %s", app.Name, row.LatestKnownTag),
			map[string]string{"app": app.Name, "tag": row.LatestKnownTag})
	}

	return row
}

func checkStandaloneAppForUpdates(ctx context.Context, app model.MyAppList, cfg *autoupdate.Config) AutoUpdateAppStatus {
	_, currentTag := docker.ExtractImageAndTag(app.Image)
	settings := autoupdate.SettingsFor(cfg, app.Name)

	row := AutoUpdateAppStatus{
		Name: app.Name,
		// v1's app.Name is already resolved via GetContainerAppList's own
		// "name" label preference (falls back to the container name) - no
		// separate store-title lookup needed here like the v2 path has.
		DisplayName:    app.Name,
		AppType:        "v1",
		CurrentImage:   app.Image,
		CurrentTag:     currentTag,
		AutoUpdate:     settings.AutoUpdate,
		Notify:         settings.Notify,
		IsUncontrolled: app.IsUncontrolled,
	}

	newTag, ok := checkNewestTag(ctx, app.Image, currentTag)
	if !ok {
		return carryForwardKnownUpdate(row)
	}
	row.LatestKnownTag = newTag

	if newTag != currentTag {
		row.UpdateAvailable = true
		// see the matching comment in checkComposeAppForUpdates - an
		// auto-update app updates itself on this same pass, so "available"
		// would just be a redundant ping seconds before "updated".
		if settings.Notify && !settings.AutoUpdate {
			notifyImageUpdate(app.Name, row.DisplayName, app.Name, newTag)
		}
	}

	return row
}

// applyStandaloneAppUpdate reuses the same export -> archive -> install
// pipeline the frontend's manual "Rebuild" action already drives
// (ToComposeYAML -> ArchiveContainer -> InstallComposeApp), patching the
// image tag into the exported compose struct before installing. This has
// the known, already-accepted side effect of permanently promoting the
// container into an ongoing-manageable compose app - exactly what manual
// Rebuild already does, just triggered automatically here.
func applyStandaloneAppUpdate(ctx context.Context, containerID, newImage string) error {
	info, err := MyService.Docker().DescribeContainer(ctx, containerID)
	if err != nil {
		return err
	}

	customizationData := v1.GetCustomizationPostData(*info)
	composeAppData := customizationData.Compose()
	composeApp := (*ComposeApp)(&composeAppData)

	for i := range composeApp.Services {
		composeApp.Services[i].Image = newImage
	}

	if err := MyService.Docker().StopContainer(containerID); err != nil {
		return err
	}
	container, err := MyService.Docker().GetContainer(containerID)
	if err != nil {
		return err
	}
	if err := MyService.Docker().RenameContainer(container.Names[0]+"_old", containerID); err != nil {
		return err
	}

	return MyService.Compose().Install(ctx, composeApp)
}

func checkNewestTag(ctx context.Context, image, currentTag string) (string, bool) {
	currentVersion, err := semver.NewVersion(currentTag)
	if err != nil {
		// the currently pinned tag isn't a version at all (latest, main, a
		// git sha, ...) - there's no valid basis to call any registry tag
		// "newer" than that, so don't guess. Without this check, an app
		// pinned to :latest would get compared against every dated/numbered
		// tag still sitting in the registry's tag list and could get
		// "newer" reported for a tag that's actually years old (confirmed:
		// this reported Home Assistant's 2024.4.4 as newer than :latest).
		return "", false
	}

	tags, err := docker.GetTags(ctx, image)
	if err != nil {
		logger.Error("autoupdate: failed to list registry tags", zap.Error(err), zap.String("image", image))
		return "", false
	}

	return autoupdate.NewestTag(tags, currentVersion.Prerelease() != "")
}

// firstAutoUpdateAttempt reports whether this is the first time this cron
// has tried to auto-update appName to tag, and marks it tried - so a pull
// that fails every hour only pings "Updating"/"Update failed" once instead
// of re-announcing the same stuck attempt on every retry. A later success
// against the same tag still always announces "Updated" regardless (see
// callers) - only the noisy "still trying" pings are deduped, not the
// terminal outcome once it actually changes.
func firstAutoUpdateAttempt(appName, tag string) bool {
	tracker := sharedNotifiedTracker()
	key := appName + ":auto-update-attempt:" + tag
	if tracker.AlreadyNotified(key) {
		return false
	}
	tracker.MarkNotified(key)
	return true
}

func notifyImageUpdate(appName, displayName, serviceName, newTag string) {
	key := appName + ":" + serviceName + ":" + newTag
	tracker := sharedNotifiedTracker()
	if tracker.AlreadyNotified(key) {
		return
	}
	tracker.MarkNotified(key)
	webhook.Send("image_update", "Update available",
		fmt.Sprintf("A new image version (%s) is available for %s", newTag, displayName),
		map[string]string{"app": appName, "service": serviceName, "tag": newTag})
}
