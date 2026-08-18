package service

import (
	"context"
	"fmt"
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
)

// GetAutoUpdateStatus returns the last-computed status list, populated by
// the cron tick (see CheckAndApplyAutoUpdates) - the API handler reads
// this rather than hitting registries live on every page load.
func GetAutoUpdateStatus() []AutoUpdateAppStatus {
	autoUpdateStatusMu.Lock()
	defer autoUpdateStatusMu.Unlock()
	out := make([]AutoUpdateAppStatus, len(autoUpdateStatusCache))
	copy(out, autoUpdateStatusCache)
	return out
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
func CheckAndApplyAutoUpdates(ctx context.Context, notified *sync.Map) {
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
		updateAutoUpdateStatusCacheEntry(checkAndMaybeApplyComposeApp(ctx, app, cfg, notified))
		time.Sleep(pacingDelay)
	}

	casaOSApps, _ := MyService.Docker().GetContainerAppList(nil, nil, nil)
	if casaOSApps != nil {
		for _, app := range *casaOSApps {
			updateAutoUpdateStatusCacheEntry(checkAndMaybeApplyStandaloneApp(ctx, app, cfg, notified))
			time.Sleep(pacingDelay)
		}
	}
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
			row := checkComposeApp(ctx, app, cfg)
			updateAutoUpdateStatusCacheEntry(row)
			return row, nil
		}
	}

	casaOSApps, _ := MyService.Docker().GetContainerAppList(nil, nil, nil)
	if casaOSApps != nil {
		for _, app := range *casaOSApps {
			if app.Name == appName {
				row := checkStandaloneApp(ctx, app, cfg)
				updateAutoUpdateStatusCacheEntry(row)
				return row, nil
			}
		}
	}

	return AutoUpdateAppStatus{}, fmt.Errorf("app %q not found among managed apps", appName)
}

func checkAndMaybeApplyComposeApp(ctx context.Context, app *ComposeApp, cfg *autoupdate.Config, notified *sync.Map) AutoUpdateAppStatus {
	row, newImageByService := checkComposeAppForUpdates(ctx, app, cfg, notified)

	if !row.IsUncontrolled && len(newImageByService) > 0 && row.AutoUpdate {
		if err := app.UpdateImages(ctx, newImageByService); err != nil {
			logger.Error("autoupdate: failed to apply compose app update", zap.Error(err), zap.String("app", app.Name))
			webhook.Send("image_update_failed", "Auto-update failed",
				fmt.Sprintf("Failed to auto-update %s: %s", row.DisplayName, err.Error()),
				map[string]string{"app": app.Name})
		} else {
			webhook.Send("image_update_applied", "Auto-update applied",
				fmt.Sprintf("%s was auto-updated to %s", row.DisplayName, row.LatestKnownTag),
				map[string]string{"app": app.Name, "tag": row.LatestKnownTag})
		}
	}

	return row
}

// checkComposeApp is the read-only half of checkAndMaybeApplyComposeApp,
// used by RecheckApp (which must never apply an update, only report).
func checkComposeApp(ctx context.Context, app *ComposeApp, cfg *autoupdate.Config) AutoUpdateAppStatus {
	row, _ := checkComposeAppForUpdates(ctx, app, cfg, &sync.Map{})
	return row
}

func checkComposeAppForUpdates(ctx context.Context, app *ComposeApp, cfg *autoupdate.Config, notified *sync.Map) (AutoUpdateAppStatus, map[string]string) {
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

	for _, svc := range app.Services {
		img, currentTag := docker.ExtractImageAndTag(svc.Image)

		if row.CurrentImage == "" {
			row.CurrentImage = svc.Image
			row.CurrentTag = currentTag
		}

		if uncontrolled {
			continue
		}

		newTag, ok := checkNewestTag(ctx, svc.Image, currentTag)
		if !ok {
			continue
		}

		if newTag != currentTag {
			// prefer surfacing a service that actually has an update in
			// the summary row, overwriting the "first service" default
			row.CurrentImage = svc.Image
			row.CurrentTag = currentTag
			row.LatestKnownTag = newTag
			row.UpdateAvailable = true
			newImageByService[svc.Name] = img + ":" + newTag
			if settings.Notify {
				notifyImageUpdate(app.Name, row.DisplayName, svc.Name, newTag, notified)
			}
		} else if row.LatestKnownTag == "" {
			row.LatestKnownTag = newTag
		}
	}

	return row, newImageByService
}

// composeAppDisplayName mirrors the same Title resolution the dashboard's
// own app grid uses (WebAppGridItemAdapterV2) - falls back to the raw
// compose project name (e.g. a random "friendly_jeanie"-style name) when
// no nicer title was ever stored, same as the dashboard does for that case.
func composeAppDisplayName(app *ComposeApp) string {
	storeInfo, err := app.StoreInfo(false)
	if err != nil || storeInfo == nil {
		return app.Name
	}
	if title, ok := storeInfo.Title[common.DefaultLanguage]; ok && title != "" {
		return title
	}
	return app.Name
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

func checkAndMaybeApplyStandaloneApp(ctx context.Context, app model.MyAppList, cfg *autoupdate.Config, notified *sync.Map) AutoUpdateAppStatus {
	row := checkStandaloneAppForUpdates(ctx, app, cfg, notified)

	if !row.UpdateAvailable || row.IsUncontrolled || !row.AutoUpdate {
		return row
	}

	img, _ := docker.ExtractImageAndTag(app.Image)
	if err := applyStandaloneAppUpdate(ctx, app.ID, img+":"+row.LatestKnownTag); err != nil {
		logger.Error("autoupdate: failed to apply standalone app update", zap.Error(err), zap.String("app", app.Name))
		webhook.Send("image_update_failed", "Auto-update failed",
			fmt.Sprintf("Failed to auto-update %s: %s", app.Name, err.Error()),
			map[string]string{"app": app.Name})
	} else {
		// unlike the compose path, Install() only synchronously validates
		// and writes the new compose file before returning - the actual
		// pull+start happens in its own background goroutine (same
		// fire-and-forget shape the manual "Rebuild" action already has
		// today). This "applied" notification reflects that the promotion
		// to a compose app was accepted, not a fully confirmed running
		// container - a real failure past this point would still surface
		// via CasaOS's own EventTypeAppInstallError message-bus event.
		webhook.Send("image_update_applied", "Auto-update applied",
			fmt.Sprintf("%s was auto-updated to %s", app.Name, row.LatestKnownTag),
			map[string]string{"app": app.Name, "tag": row.LatestKnownTag})
	}

	return row
}

func checkStandaloneApp(ctx context.Context, app model.MyAppList, cfg *autoupdate.Config) AutoUpdateAppStatus {
	return checkStandaloneAppForUpdates(ctx, app, cfg, &sync.Map{})
}

func checkStandaloneAppForUpdates(ctx context.Context, app model.MyAppList, cfg *autoupdate.Config, notified *sync.Map) AutoUpdateAppStatus {
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

	if app.IsUncontrolled {
		return row
	}

	newTag, ok := checkNewestTag(ctx, app.Image, currentTag)
	if !ok {
		return row
	}
	row.LatestKnownTag = newTag

	if newTag != currentTag {
		row.UpdateAvailable = true
		if settings.Notify {
			notifyImageUpdate(app.Name, row.DisplayName, app.Name, newTag, notified)
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

func notifyImageUpdate(appName, displayName, serviceName, newTag string, notified *sync.Map) {
	key := appName + ":" + serviceName + ":" + newTag
	if _, already := notified.Load(key); already {
		return
	}
	notified.Store(key, true)
	webhook.Send("image_update", "Update available",
		fmt.Sprintf("A new image version (%s) is available for %s", newTag, displayName),
		map[string]string{"app": appName, "service": serviceName, "tag": newTag})
}
