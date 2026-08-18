package service

import (
	"context"
	"fmt"
	"sync"
	"time"

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
// do, since the UI's policy control is per-app, not per-service).
type AutoUpdateAppStatus struct {
	Name            string            `json:"name"`
	AppType         string            `json:"appType"` // "v1" | "v2"
	CurrentImage    string            `json:"currentImage"`
	CurrentTag      string            `json:"currentTag"`
	LatestKnownTag  string            `json:"latestKnownTag"`
	UpdateAvailable bool              `json:"updateAvailable"`
	Policy          autoupdate.Policy `json:"policy"`
	IsUncontrolled  bool              `json:"isUncontrolled"`
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

func setAutoUpdateStatusCache(status []AutoUpdateAppStatus) {
	autoUpdateStatusMu.Lock()
	defer autoUpdateStatusMu.Unlock()
	autoUpdateStatusCache = status
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

	var status []AutoUpdateAppStatus

	composeApps, err := MyService.Compose().List(ctx)
	if err != nil {
		logger.Error("autoupdate: failed to list compose apps", zap.Error(err))
	}
	for _, app := range composeApps {
		status = append(status, checkAndMaybeApplyComposeApp(ctx, app, cfg, notified))
		time.Sleep(pacingDelay)
	}

	casaOSApps, _ := MyService.Docker().GetContainerAppList(nil, nil, nil)
	if casaOSApps != nil {
		for _, app := range *casaOSApps {
			status = append(status, checkAndMaybeApplyStandaloneApp(ctx, app, cfg, notified))
			time.Sleep(pacingDelay)
		}
	}

	setAutoUpdateStatusCache(status)
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

	if !row.IsUncontrolled && len(newImageByService) > 0 && row.Policy == autoupdate.PolicyAuto {
		if err := app.UpdateImages(ctx, newImageByService); err != nil {
			logger.Error("autoupdate: failed to apply compose app update", zap.Error(err), zap.String("app", app.Name))
			webhook.Send("image_update_failed", "Auto-update failed",
				fmt.Sprintf("Failed to auto-update %s: %s", app.Name, err.Error()),
				map[string]string{"app": app.Name})
		} else {
			webhook.Send("image_update_applied", "Auto-update applied",
				fmt.Sprintf("%s was auto-updated to %s", app.Name, row.LatestKnownTag),
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
	policy := autoupdate.PolicyFor(cfg, app.Name)

	row := AutoUpdateAppStatus{
		Name:           app.Name,
		AppType:        "v2",
		Policy:         policy,
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
			notifyImageUpdate(app.Name, svc.Name, newTag, notified)
		} else if row.LatestKnownTag == "" {
			row.LatestKnownTag = newTag
		}
	}

	return row, newImageByService
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

	if !row.UpdateAvailable || row.IsUncontrolled || row.Policy != autoupdate.PolicyAuto {
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

	row := AutoUpdateAppStatus{
		Name:           app.Name,
		AppType:        "v1",
		CurrentImage:   app.Image,
		CurrentTag:     currentTag,
		Policy:         autoupdate.PolicyFor(cfg, app.Name),
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
		notifyImageUpdate(app.Name, app.Name, newTag, notified)
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
	tags, err := docker.GetTags(ctx, image)
	if err != nil {
		logger.Error("autoupdate: failed to list registry tags", zap.Error(err), zap.String("image", image))
		return "", false
	}

	includePrerelease := false
	if v, err := semver.NewVersion(currentTag); err == nil && v.Prerelease() != "" {
		includePrerelease = true
	}

	return autoupdate.NewestTag(tags, includePrerelease)
}

func notifyImageUpdate(appName, serviceName, newTag string, notified *sync.Map) {
	key := appName + ":" + serviceName + ":" + newTag
	if _, already := notified.Load(key); already {
		return
	}
	notified.Store(key, true)
	webhook.Send("image_update", "Update available",
		fmt.Sprintf("A new image version (%s) is available for %s", newTag, appName),
		map[string]string{"app": appName, "service": serviceName, "tag": newTag})
}
