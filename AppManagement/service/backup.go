package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/CasaOS-AppManagement/common"
	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/autoupdate"
	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/webhook"
	v1 "github.com/IceWhaleTech/CasaOS-AppManagement/service/v1"
	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/IceWhaleTech/CasaOS-Common/utils/port"
	jsoniter "github.com/json-iterator/go"
	uuid "github.com/satori/go.uuid"
	"go.uber.org/zap"
)

// ForkVersion is this fork's release tag, set once at startup by main.go
// (same pattern as webhook.Version) - stamped into exported manifests so a
// restored archive's origin is traceable.
var ForkVersion string

const backupFormatVersion = "1"

// stagingRoot is where an uploaded archive is fully extracted before
// anything touches a live path - see ImportBackup. A var, not a const, so
// tests can point it at a temp directory.
var stagingRoot = filepath.Join(constants.DefaultDataPath, "backup-staging")

// BackupManifest is the first entry written into an export archive and the
// first thing Import reads, so it can check available disk space before
// staging any file content.
type BackupManifest struct {
	FormatVersion  string           `json:"format_version"`
	CreatedAt      time.Time        `json:"created_at"`
	ForkVersion    string           `json:"fork_version"`
	TotalDataBytes int64            `json:"total_data_bytes"`
	Apps           []BackupAppEntry `json:"apps"`

	// UserCustom is opaque per-user data (folder groupings, dashboard
	// order) that AppManagement has no way to fetch itself - it lives in a
	// separate microservice this package has no client for. The frontend
	// fetches it and passes it in at export time; on import it's handed
	// back verbatim for the frontend to write back out. Never interpreted
	// here.
	UserCustom jsoniter.RawMessage `json:"user_custom,omitempty"`
}

type BackupAppEntry struct {
	Name        string   `json:"name"`
	SourceType  string   `json:"source_type"` // "compose" | "standalone"
	ComposePath string   `json:"compose_path"`
	DataPaths   []string `json:"data_paths"` // absolute host paths bundled for this app
}

type ImportResult struct {
	Imported []string     `json:"imported"`
	Skipped  []SkippedApp `json:"skipped"`
	Failed   []FailedApp  `json:"failed"`

	// UserCustom is the archive's UserCustom blob, passed straight through
	// unexamined - the frontend already has the app names that actually
	// landed in Imported and can merge this in itself.
	UserCustom jsoniter.RawMessage `json:"user_custom,omitempty"`
}

type SkippedApp struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type FailedApp struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// backupApp is one app normalized for the export walk - both v1 standalone
// containers and v2 compose apps end up in this same shape.
type backupApp struct {
	name        string
	displayName string
	sourceType  string
	composeYAML []byte
	dataPaths   []string
}

// ExportBackup streams every managed app's compose config, bind-mounted
// data, and the shared webhook/autoupdate config as a single tar.gz written
// directly to w - no buffering of the whole archive in memory or on disk.
// excludeData names apps whose compose config should still be included but
// whose data directories should be skipped (the user transferring that
// data some other way) - nil or empty includes every app's data as before.
// userCustom is opaque per-user data (folder groupings, dashboard order)
// the frontend fetched from the service that actually owns it and passed
// straight through - see BackupManifest.UserCustom.
func ExportBackup(ctx context.Context, w io.Writer, excludeData map[string]bool, userCustom jsoniter.RawMessage) error {
	apps, err := collectBackupApps(ctx)
	if err != nil {
		return err
	}
	for i := range apps {
		if excludeData[apps[i].name] {
			apps[i].dataPaths = nil
		}
	}

	manifest := BackupManifest{
		FormatVersion: backupFormatVersion,
		CreatedAt:     time.Now().UTC(),
		ForkVersion:   ForkVersion,
		UserCustom:    userCustom,
	}
	for _, a := range apps {
		manifest.Apps = append(manifest.Apps, BackupAppEntry{
			Name:        a.name,
			SourceType:  a.sourceType,
			ComposePath: composeArchivePath(a.name),
			DataPaths:   a.dataPaths,
		})
		for _, dataPath := range a.dataPaths {
			size, err := dirSize(dataPath)
			if err != nil {
				logger.Error("backup: failed to size data path, excluding from manifest total", zap.Error(err), zap.String("path", dataPath))
				continue
			}
			manifest.TotalDataBytes += size
		}
	}

	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)

	if err := writeManifestEntry(tw, manifest); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	// A single app's data - one unreadable file, a permission error, a
	// socket or device file WalkDir chokes on - must not sacrifice every
	// other app already queued behind it. Headers (200 OK) are already
	// committed by the time any of this runs (see BackupExport), so an
	// early return here doesn't fail the request - it silently truncates
	// the download with no indication anything went wrong, discovered
	// only later as an "unexpected EOF" trying to import that (genuinely
	// incomplete) file. Confirmed live: a 2KB export from a 40+ app
	// install. Import already tolerates a manifest entry whose compose
	// file or data turned out missing from the archive (reported as a
	// per-app Failed/skip, not an aborted import), so skipping here is
	// safe on that side too.
	for _, a := range apps {
		if err := writeBytesEntry(tw, a.composeYAML, composeArchivePath(a.name)); err != nil {
			logger.Error("backup: failed to write compose file, skipping app", zap.Error(err), zap.String("app", a.name))
			continue
		}
		for _, dataPath := range a.dataPaths {
			if err := writeDirEntries(tw, dataPath, dataArchivePath(dataPath)); err != nil {
				logger.Error("backup: failed to archive app data, app will import with no data", zap.Error(err), zap.String("app", a.name), zap.String("path", dataPath))
			}
		}
	}

	if err := writeConfigFileEntry(tw, webhook.ConfigFilePath, "config/webhooks.json"); err != nil {
		logger.Error("backup: failed to write webhooks config", zap.Error(err))
	}
	if err := writeConfigFileEntry(tw, autoupdate.ConfigFilePath, "config/autoupdate.json"); err != nil {
		logger.Error("backup: failed to write autoupdate config", zap.Error(err))
	}

	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// collectBackupApps normalizes every CasaOS-managed app - v2 compose apps
// and v1 standalone containers alike - into the same backupApp shape.
func collectBackupApps(ctx context.Context) ([]backupApp, error) {
	var apps []backupApp

	composeApps, err := MyService.Compose().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing compose apps: %w", err)
	}
	for _, composeApp := range composeApps {
		ba, err := composeAppToBackupApp(composeApp)
		if err != nil {
			logger.Error("backup: failed to prepare compose app for export, skipping", zap.Error(err), zap.String("app", composeApp.Name))
			continue
		}
		apps = append(apps, ba)
	}

	if casaOSApps, _ := MyService.Docker().GetContainerAppList(nil, nil, nil); casaOSApps != nil {
		for _, app := range *casaOSApps {
			ba, err := standaloneContainerToBackupApp(ctx, app.ID)
			if err != nil {
				logger.Error("backup: failed to prepare standalone app for export, skipping", zap.Error(err), zap.String("app", app.Name))
				continue
			}
			apps = append(apps, ba)
		}
	}

	return apps, nil
}

// BackupAppSummary is what GET /v1/backup/apps serves - lets the UI show a
// per-app "include data" checklist before export actually starts, without
// archiving anything.
type BackupAppSummary struct {
	Name          string   `json:"name"`
	DisplayName   string   `json:"display_name"`
	SourceType    string   `json:"source_type"`
	DataPaths     []string `json:"data_paths"`
	DataSizeBytes int64    `json:"data_size_bytes"`
}

func ListBackupApps(ctx context.Context) ([]BackupAppSummary, error) {
	apps, err := collectBackupApps(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]BackupAppSummary, 0, len(apps))
	for _, a := range apps {
		var size int64
		for _, dataPath := range a.dataPaths {
			s, err := dirSize(dataPath)
			if err != nil {
				logger.Error("backup: failed to size data path", zap.Error(err), zap.String("path", dataPath))
				continue
			}
			size += s
		}
		summaries = append(summaries, BackupAppSummary{
			Name:          a.name,
			DisplayName:   a.displayName,
			SourceType:    a.sourceType,
			DataPaths:     a.dataPaths,
			DataSizeBytes: size,
		})
	}
	return summaries, nil
}

func composeAppToBackupApp(composeApp *ComposeApp) (backupApp, error) {
	yamlBytes, err := GenerateYAMLFromComposeApp(*composeApp)
	if err != nil {
		return backupApp{}, err
	}

	dataPaths := bindMountSourcesOf(composeApp)
	if storeInfo, err := composeApp.StoreInfo(false); err == nil && storeInfo != nil {
		if iconPath, ok := customIconPathFromURL(storeInfo.Icon); ok {
			if _, statErr := os.Stat(iconPath); statErr == nil {
				// Not a bind mount - nothing mounts this file into any
				// container - but restoring it to this exact same absolute
				// path is exactly what every dataPaths entry already does,
				// and x-casaos.icon (already captured as-is in composeYAML
				// above) points at this literal path, so no separate
				// archive section or URL rewriting is needed for the icon
				// to keep working after import.
				dataPaths = append(dataPaths, iconPath)
			}
		}
	}

	return backupApp{
		name:        composeApp.Name,
		displayName: composeAppDisplayName(composeApp),
		sourceType:  "compose",
		composeYAML: yamlBytes,
		dataPaths:   dataPaths,
	}, nil
}

// customIconSubdir mirrors the constant of the same name in the main CasaOS
// module (service/system.go) - AppManagement has no client for that
// service, but both run on the same host and share the same filesystem, so
// reading the icon file directly is enough; no cross-service call needed.
const customIconSubdir = "casaos-custom-icons"

// customIconPathFromURL extracts the absolute host file path behind an
// x-casaos.icon value that points at this server's own custom-icon store -
// a URL of the form "/v1/custom-icons?path=<abs path>" written by the icon
// upload flow (UI/src/components/Apps/ComposeConfig.vue) - and reports
// ok=false for anything else (an external icon.casaos.io/Docker Hub URL, or
// no icon set at all). Those need nothing done to survive export/import,
// since they're already portable as a plain URL.
func customIconPathFromURL(iconURL string) (path string, ok bool) {
	if iconURL == "" {
		return "", false
	}
	u, err := url.Parse(iconURL)
	if err != nil || !strings.HasSuffix(u.Path, "/custom-icons") {
		return "", false
	}
	raw := u.Query().Get("path")
	if raw == "" {
		return "", false
	}
	clean := filepath.Clean(raw)
	if !filepath.IsAbs(clean) || filepath.Base(filepath.Dir(clean)) != customIconSubdir {
		return "", false
	}
	return clean, true
}

// standaloneContainerToBackupApp converts a v1 standalone container to the
// same compose shape applyStandaloneAppUpdate uses (DescribeContainer ->
// GetCustomizationPostData -> Compose()), but stops there - unlike that
// caller, this must stay read-only and never stop/rename the container.
func standaloneContainerToBackupApp(ctx context.Context, containerID string) (backupApp, error) {
	info, err := MyService.Docker().DescribeContainer(ctx, containerID)
	if err != nil {
		return backupApp{}, err
	}

	customizationData := v1.GetCustomizationPostData(*info)
	composeAppData := customizationData.Compose()
	composeApp := (*ComposeApp)(&composeAppData)

	ba, err := composeAppToBackupApp(composeApp)
	if err != nil {
		return backupApp{}, err
	}
	ba.sourceType = "standalone"
	return ba, nil
}

// bindMountSourcesOf returns the absolute host paths bind-mounted by
// composeApp's services, deduplicated. Reads Type=="bind" off the generated
// compose struct rather than raw Docker mount metadata, so "what we
// bundled" stays consistent with "what will actually get bind-mounted on
// reinstall" - CasaOS's own bind/volume classification
// (model.PathArray.ServiceVolumeConfigList) is a heuristic on the path
// string, not a copy of Docker's real mount type.
func bindMountSourcesOf(composeApp *ComposeApp) []string {
	seen := map[string]bool{}
	var paths []string
	for _, svc := range composeApp.Services {
		for _, vol := range svc.Volumes {
			if vol.Type != "bind" || vol.Source == "" || seen[vol.Source] || isHostSystemPassthrough(vol.Source) {
				continue
			}
			seen[vol.Source] = true
			paths = append(paths, vol.Source)
		}
	}
	return paths
}

// hostSystemPassthroughExact and hostSystemPassthroughPrefixes list bind
// sources that are host runtime/system resources, not application data -
// a dbus or docker socket, device nodes, or an /etc file a compose file
// passes straight through from the host (e.g. Home Assistant's
// /run/dbus:/run/dbus mount). Every one of these already exists, or gets
// recreated by the OS, on any host at boot - archiving one and restoring
// it onto a different host's live copy is never correct, and collides with
// what's already there.
var hostSystemPassthroughExact = map[string]bool{
	"/run":             true,
	"/var/run":         true,
	"/dev":             true,
	"/proc":            true,
	"/sys":             true,
	"/etc/localtime":   true,
	"/etc/timezone":    true,
	"/etc/hosts":       true,
	"/etc/resolv.conf": true,
}

var hostSystemPassthroughPrefixes = []string{
	"/run/",
	"/var/run/",
	"/dev/",
	"/proc/",
	"/sys/",
}

func isHostSystemPassthrough(absSource string) bool {
	if hostSystemPassthroughExact[absSource] {
		return true
	}
	for _, prefix := range hostSystemPassthroughPrefixes {
		if strings.HasPrefix(absSource, prefix) {
			return true
		}
	}
	return false
}

func composeArchivePath(appName string) string {
	return "compose/" + appName + "/docker-compose.yml"
}

// dataArchivePath mirrors the literal absolute source path under appdata/,
// stripped of its leading slash - restored to that exact same absolute
// path on import regardless of whether /DATA/AppData convention holds on
// either box, since the path always comes from the compose YAML itself,
// never a hardcoded guess.
func dataArchivePath(absSource string) string {
	return "appdata/" + strings.TrimPrefix(filepath.ToSlash(absSource), "/")
}

func writeManifestEntry(tw *tar.Writer, manifest BackupManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return writeBytesEntry(tw, data, "manifest.json")
}

func writeBytesEntry(tw *tar.Writer, data []byte, name string) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data))}); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func writeConfigFileEntry(tw *tar.Writer, srcPath, archiveName string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing configured yet - not an error
		}
		return err
	}
	return writeBytesEntry(tw, data, archiveName)
}

func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// writeDirEntries walks root and writes one tar entry per file/dir/symlink
// under archivePrefix, preserving ownership and never following symlinks
// (avoids infinite loops on cyclic links and avoids silently duplicating
// data a symlink points to outside the bind-mount root).
//
// A single unreadable file (permission denied, a socket/device file, a
// race with something deleting it mid-walk) is logged and skipped rather
// than aborting the whole directory - critically, a regular file is
// os.Open'd BEFORE its tar.Header is written, not after. tar.Writer
// enforces that each entry's declared Size is fully written before the
// next WriteHeader call; committing a header and then failing to open the
// file would desync every entry written after it for the rest of this
// export, not just this one file.
func writeDirEntries(tw *tar.Writer, root, archivePrefix string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			logger.Error("backup: failed to walk path, skipping", zap.Error(walkErr), zap.String("path", path))
			return nil
		}

		info, err := d.Info()
		if err != nil {
			logger.Error("backup: failed to stat path, skipping", zap.Error(err), zap.String("path", path))
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			logger.Error("backup: failed to compute archive path, skipping", zap.Error(err), zap.String("path", path))
			return nil
		}
		archiveName := archivePrefix
		if rel != "." {
			archiveName = archivePrefix + "/" + filepath.ToSlash(rel)
		}

		var link string
		if info.Mode()&os.ModeSymlink != 0 {
			if link, err = os.Readlink(path); err != nil {
				logger.Error("backup: failed to read symlink, skipping", zap.Error(err), zap.String("path", path))
				return nil
			}
		}

		// open before WriteHeader, per the func comment above
		var f *os.File
		if info.Mode().IsRegular() {
			if f, err = os.Open(path); err != nil {
				logger.Error("backup: failed to open file, skipping", zap.Error(err), zap.String("path", path))
				return nil
			}
			defer f.Close()
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			logger.Error("backup: failed to build tar header, skipping", zap.Error(err), zap.String("path", path))
			return nil
		}
		header.Name = archiveName
		setTarOwnership(header, info)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if f == nil {
			return nil
		}
		// a failure here (a genuine mid-read I/O error - the file having
		// opened successfully makes this far less likely than the open
		// failing outright) does desync the rest of the stream; unlike
		// every skip above, this one has to propagate.
		_, err = io.Copy(tw, f)
		return err
	})
}

// ImportPreview is what ImportBackupPreview returns and what the caller
// echoes back (with edits) to ImportBackupConfirm.
type ImportPreview struct {
	PreviewID string             `json:"preview_id"`
	Apps      []ImportAppPreview `json:"apps"`

	// UserCustom is the archive's UserCustom blob (see BackupManifest),
	// echoed back unexamined so the frontend can preview/merge it - it's
	// also echoed a second time in ImportResult once confirm actually
	// applies the import, since which apps ended up Imported isn't known
	// until then.
	UserCustom jsoniter.RawMessage `json:"user_custom,omitempty"`
}

type ImportAppPreview struct {
	Name         string                 `json:"name"`
	DisplayName  string                 `json:"display_name"`
	SourceType   string                 `json:"source_type"`
	NameConflict bool                   `json:"name_conflict"` // an app with this name already exists on this server
	HasData      bool                   `json:"has_data"`
	Services     []ImportServicePreview `json:"services"`
}

type ImportServicePreview struct {
	ServiceName string                `json:"service_name"`
	Ports       []ImportPortPreview   `json:"ports"`
	Volumes     []ImportVolumePreview `json:"volumes"`
	Env         []ImportEnvPreview    `json:"env"`
}

type ImportPortPreview struct {
	Target    uint32 `json:"target"`    // container port
	Published string `json:"published"` // host port, as originally exported
	Protocol  string `json:"protocol"`
	Conflict  bool   `json:"conflict"` // true if Published is already in use on this server
}

type ImportVolumePreview struct {
	Target string `json:"target"` // container path
	Source string `json:"source"` // host path, as originally exported
}

type ImportEnvPreview struct {
	Key   string `json:"key"`
	Value string `json:"value"` // as originally exported; a variable with no default (pass-through from the host shell) shows as empty
}

// ImportBackupPreview stages an uploaded archive under a fresh directory
// and reports what it contains - per app, per service, every port and
// bind-mounted volume, whether the app's name already collides with
// something on this server, and whether any of its ports are already in
// use - without installing anything or touching a single live path.
//
// Unlike the old one-shot import, the staging directory is deliberately
// NOT cleaned up here - ImportBackupConfirm (keyed by the PreviewID this
// returns) does that once it's actually applied, and SweepStaleImportPreviews
// (wired into the cron in main.go) cleans up anything abandoned without
// ever being confirmed.
func ImportBackupPreview(ctx context.Context, r io.Reader) (*ImportPreview, error) {
	previewID := uuid.NewV4().String()
	sessionDir := filepath.Join(stagingRoot, previewID)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating staging directory: %w", err)
	}

	manifest, err := extractArchive(r, sessionDir)
	if err != nil {
		if rmErr := os.RemoveAll(sessionDir); rmErr != nil {
			logger.Error("backup: failed to clean up staging directory after a failed extract", zap.Error(rmErr), zap.String("path", sessionDir))
		}
		return nil, fmt.Errorf("extracting archive: %w", err)
	}

	existingCompose, existingStandalone := existingAppNames(ctx)

	tcpInUse, udpInUse, err := port.ListPortsInUse()
	if err != nil {
		logger.Error("backup: failed to list ports in use, conflict detection will be incomplete", zap.Error(err))
	}
	tcpInUseSet, udpInUseSet := toPortSet(tcpInUse), toPortSet(udpInUse)

	preview := &ImportPreview{PreviewID: previewID, UserCustom: manifest.UserCustom}
	for _, entry := range manifest.Apps {
		appPreview := ImportAppPreview{
			Name:         entry.Name,
			DisplayName:  entry.Name,
			SourceType:   entry.SourceType,
			NameConflict: existingCompose[entry.Name] || existingStandalone[entry.Name],
			HasData:      len(entry.DataPaths) > 0,
		}

		composeApp, err := readStagedComposeApp(sessionDir, entry.ComposePath)
		if err != nil {
			logger.Error("backup: failed to read staged compose file for preview", zap.Error(err), zap.String("app", entry.Name))
			preview.Apps = append(preview.Apps, appPreview)
			continue
		}
		appPreview.DisplayName = composeAppDisplayName(composeApp)

		for _, svc := range composeApp.Services {
			svcPreview := ImportServicePreview{ServiceName: svc.Name}
			for _, p := range svc.Ports {
				inUse := tcpInUseSet[p.Published]
				if strings.EqualFold(p.Protocol, "udp") {
					inUse = udpInUseSet[p.Published]
				}
				svcPreview.Ports = append(svcPreview.Ports, ImportPortPreview{
					Target: p.Target, Published: p.Published, Protocol: p.Protocol, Conflict: inUse,
				})
			}
			for _, vol := range svc.Volumes {
				if vol.Type != "bind" {
					continue
				}
				svcPreview.Volumes = append(svcPreview.Volumes, ImportVolumePreview{Target: vol.Target, Source: vol.Source})
			}
			envKeys := make([]string, 0, len(svc.Environment))
			for k := range svc.Environment {
				envKeys = append(envKeys, k)
			}
			sort.Strings(envKeys) // compose-go's Environment is a map - sort for a stable review-screen order
			for _, k := range envKeys {
				value := ""
				if v := svc.Environment[k]; v != nil {
					value = *v
				}
				svcPreview.Env = append(svcPreview.Env, ImportEnvPreview{Key: k, Value: value})
			}
			appPreview.Services = append(appPreview.Services, svcPreview)
		}

		preview.Apps = append(preview.Apps, appPreview)
	}

	return preview, nil
}

// existingAppNames returns every app name already installed on this
// server, split by kind, since a v1 standalone app and a v2 compose app
// share one namespace for collision purposes.
func existingAppNames(ctx context.Context) (compose map[string]bool, standalone map[string]bool) {
	compose = map[string]bool{}
	if composeApps, err := MyService.Compose().List(ctx); err != nil {
		logger.Error("backup: failed to list existing compose apps", zap.Error(err))
	} else {
		for name := range composeApps {
			compose[name] = true
		}
	}

	standalone = map[string]bool{}
	if casaOSApps, _ := MyService.Docker().GetContainerAppList(nil, nil, nil); casaOSApps != nil {
		for _, app := range *casaOSApps {
			standalone[app.Name] = true
		}
	}
	return compose, standalone
}

func toPortSet(ports []int) map[string]bool {
	set := make(map[string]bool, len(ports))
	for _, p := range ports {
		set[strconv.Itoa(p)] = true
	}
	return set
}

func readStagedComposeApp(sessionDir, composeArchiveRelPath string) (*ComposeApp, error) {
	yamlBytes, err := os.ReadFile(filepath.Join(sessionDir, filepath.FromSlash(composeArchiveRelPath)))
	if err != nil {
		return nil, err
	}
	return NewComposeAppFromYAML(yamlBytes, false, true)
}

// AppImportDecision is one app's worth of review-screen edits, echoed back
// from ImportBackupPreview's response to ImportBackupConfirm.
type AppImportDecision struct {
	Name    string           `json:"name"`
	Skip    bool             `json:"skip"`
	Ports   []PortOverride   `json:"ports"`
	Volumes []VolumeOverride `json:"volumes"`
	Env     []EnvOverride    `json:"env"`
}

// PortOverride identifies one port by (service, container port, protocol)
// and gives its new host port - an override with an empty Published is a
// no-op, so only edited ports need to be sent at all.
type PortOverride struct {
	ServiceName string `json:"service_name"`
	Target      uint32 `json:"target"`
	Protocol    string `json:"protocol"`
	Published   string `json:"published"`
}

// VolumeOverride identifies one bind-mounted volume by (service, container
// path) and gives its new host destination - an override with an empty
// Source is a no-op, so only redirected volumes need to be sent at all.
type VolumeOverride struct {
	ServiceName string `json:"service_name"`
	Target      string `json:"target"`
	Source      string `json:"source"`
}

// EnvOverride identifies one environment variable by (service, key) and
// gives its new value - matched against the key rather than position, since
// compose-go's Environment is a map with no inherent order.
type EnvOverride struct {
	ServiceName string `json:"service_name"`
	Key         string `json:"key"`
	Value       string `json:"value"`
}

// ImportBackupConfirm reopens the staging directory ImportBackupPreview
// created (keyed by previewID) and actually applies it: for each app not
// marked Skip, load its staged compose file, apply whatever port/volume
// overrides the review screen collected (an app absent from decisions, or
// a port/volume left unedited within one, keeps its original exported
// value), move its data from where the archive staged it to wherever it's
// actually supposed to land (the original path, unless overridden), then
// install through the same pipeline InstallComposeApp uses. Each app is
// independent - one failure doesn't block or roll back the rest. The
// staging directory is removed once every app has been processed,
// success or failure.
func ImportBackupConfirm(ctx context.Context, previewID string, decisions []AppImportDecision) (*ImportResult, error) {
	sessionDir := filepath.Join(stagingRoot, previewID)
	if info, err := os.Stat(sessionDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("preview %q not found - it may have already been confirmed, or expired", previewID)
	}
	defer func() {
		if err := os.RemoveAll(sessionDir); err != nil {
			logger.Error("backup: failed to clean up staging directory", zap.Error(err), zap.String("path", sessionDir))
		}
	}()

	manifestBytes, err := os.ReadFile(filepath.Join(sessionDir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("reading staged manifest: %w", err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("parsing staged manifest: %w", err)
	}

	restoreConfigFiles(sessionDir)

	decisionByName := make(map[string]AppImportDecision, len(decisions))
	for _, d := range decisions {
		decisionByName[d.Name] = d
	}

	result := &ImportResult{UserCustom: manifest.UserCustom}

	existingCompose, existingStandalone := existingAppNames(ctx)

	for _, entry := range manifest.Apps {
		decision, hasDecision := decisionByName[entry.Name]
		if hasDecision && decision.Skip {
			result.Skipped = append(result.Skipped, SkippedApp{Name: entry.Name, Reason: "skipped by user"})
			continue
		}

		if existingCompose[entry.Name] || existingStandalone[entry.Name] {
			result.Skipped = append(result.Skipped, SkippedApp{Name: entry.Name, Reason: "an app with this name already exists on this server"})
			continue
		}

		composeApp, err := readStagedComposeApp(sessionDir, entry.ComposePath)
		if err != nil {
			result.Failed = append(result.Failed, FailedApp{Name: entry.Name, Reason: "missing or invalid compose file in archive: " + err.Error()})
			continue
		}

		// original archived source -> actual destination, defaulting to
		// itself (unmoved) unless applyVolumeOverrides below redirects it -
		// restoreDataPathsWithOverrides needs the ORIGINAL path to find
		// what's staged on disk regardless of where it ends up.
		destinations := map[string]string{}
		for _, svc := range composeApp.Services {
			for _, vol := range svc.Volumes {
				if vol.Type == "bind" && vol.Source != "" {
					destinations[vol.Source] = vol.Source
				}
			}
		}

		if hasDecision {
			applyPortOverrides(composeApp, decision.Ports)
			applyVolumeOverrides(composeApp, decision.Volumes, destinations)
			applyEnvOverrides(composeApp, decision.Env)
		}

		validation, err := composeApp.GetPortsInUse()
		if err != nil {
			result.Failed = append(result.Failed, FailedApp{Name: entry.Name, Reason: "checking port conflicts: " + err.Error()})
			continue
		}
		if validation != nil {
			result.Skipped = append(result.Skipped, SkippedApp{Name: entry.Name, Reason: "one or more ports are still in use - go back and pick a different port"})
			continue
		}

		if err := restoreDataPathsWithOverrides(sessionDir, entry.DataPaths, destinations); err != nil {
			result.Failed = append(result.Failed, FailedApp{Name: entry.Name, Reason: "restoring app data: " + err.Error()})
			continue
		}

		installCtx := common.WithProperties(context.Background(), map[string]string{})
		if err := MyService.Compose().Install(installCtx, composeApp); err != nil {
			result.Failed = append(result.Failed, FailedApp{Name: entry.Name, Reason: "install failed: " + err.Error()})
			continue
		}

		result.Imported = append(result.Imported, entry.Name)
	}

	return result, nil
}

// applyPortOverrides rewrites Published (host) ports on composeApp to match
// the review screen's edits, matched by (service, container target,
// protocol) - an override with an empty Published is a no-op, leaving an
// unedited, non-conflicting port at its original value.
func applyPortOverrides(composeApp *ComposeApp, overrides []PortOverride) {
	for _, o := range overrides {
		if o.Published == "" {
			continue
		}
		for i := range composeApp.Services {
			if composeApp.Services[i].Name != o.ServiceName {
				continue
			}
			for j := range composeApp.Services[i].Ports {
				p := &composeApp.Services[i].Ports[j]
				if p.Target == o.Target && strings.EqualFold(p.Protocol, o.Protocol) {
					p.Published = o.Published
				}
			}
		}
	}
}

// applyVolumeOverrides rewrites a volume's Source (host path) to the
// review screen's chosen destination, matched by (service, container
// target). Records original -> new in destinations so
// restoreDataPathsWithOverrides can still find the archived data (staged
// under its ORIGINAL path) after this mutates composeApp's own copy of
// Source to the new one.
func applyVolumeOverrides(composeApp *ComposeApp, overrides []VolumeOverride, destinations map[string]string) {
	for _, o := range overrides {
		if o.Source == "" {
			continue
		}
		for i := range composeApp.Services {
			if composeApp.Services[i].Name != o.ServiceName {
				continue
			}
			for j := range composeApp.Services[i].Volumes {
				vol := &composeApp.Services[i].Volumes[j]
				if vol.Type == "bind" && vol.Target == o.Target {
					destinations[vol.Source] = o.Source
					vol.Source = o.Source
				}
			}
		}
	}
}

// applyEnvOverrides rewrites an environment variable's value to whatever
// the review screen edited it to, matched by (service, key) - a key absent
// from the service's original Environment is ignored rather than added, so
// this can only edit what was actually exported.
func applyEnvOverrides(composeApp *ComposeApp, overrides []EnvOverride) {
	for _, o := range overrides {
		if o.Value == "" {
			continue
		}
		for i := range composeApp.Services {
			if composeApp.Services[i].Name != o.ServiceName {
				continue
			}
			if _, ok := composeApp.Services[i].Environment[o.Key]; ok {
				value := o.Value
				composeApp.Services[i].Environment[o.Key] = &value
			}
		}
	}
}

// SweepStaleImportPreviews removes any staging directory under stagingRoot
// older than maxAge - a preview that got uploaded (see
// ImportBackupPreview) but never confirmed would otherwise sit there
// forever. Wired into the cron in main.go.
func SweepStaleImportPreviews(maxAge time.Duration) {
	entries, err := os.ReadDir(stagingRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Error("backup: failed to list staging root for cleanup sweep", zap.Error(err))
		}
		return
	}

	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		path := filepath.Join(stagingRoot, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			logger.Error("backup: failed to remove stale import preview", zap.Error(err), zap.String("path", path))
		} else {
			logger.Info("backup: removed a stale (unconfirmed) import preview", zap.String("path", path))
		}
	}
}

// restoreConfigFiles copies the shared webhook/autoupdate config staged
// from the archive onto this server's real config paths, overwriting
// whatever's there - reasonable given a full-server-migration import is
// meant to bring settings along too. Best-effort: a missing or unreadable
// staged file (an archive from before config files were included, or one
// that simply had none configured) just leaves this server's existing
// config untouched, not a reason to fail the whole import.
func restoreConfigFiles(sessionDir string) {
	copies := map[string]string{
		filepath.Join(sessionDir, "config", "webhooks.json"):   webhook.ConfigFilePath,
		filepath.Join(sessionDir, "config", "autoupdate.json"): autoupdate.ConfigFilePath,
	}
	for staged, dest := range copies {
		data, err := os.ReadFile(staged)
		if err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			logger.Error("backup: failed to restore config file", zap.Error(err), zap.String("path", dest))
			continue
		}
		if err := os.WriteFile(dest, data, 0o600); err != nil {
			logger.Error("backup: failed to restore config file", zap.Error(err), zap.String("path", dest))
		}
	}
}

// extractArchive stages every entry from r (gzip-wrapped tar) under dir, at
// its archive-relative path - never at a live/final path; restoreDataPaths
// is what moves things onto real paths, and only after collision checks
// pass. Every entry name is validated to resolve inside dir before writing -
// this daemon runs as root, so an untrusted or corrupted archive escaping
// the staging root is a real risk, not theoretical.
//
// ExportBackup always writes manifest.json first, so the first tar entry
// here is required to be it - its TotalDataBytes is checked against free
// space on dir's filesystem before any further entry is staged, so an
// import too large for this disk fails fast instead of filling it mid-
// transfer. The manifest is also written to dir/manifest.json (not just
// parsed and held in memory) - ImportBackupConfirm runs in a separate
// request from whatever called this, potentially long after, and needs
// to re-read it from the staged directory rather than from a variable
// that only existed for the lifetime of the preview call.
func extractArchive(r io.Reader, dir string) (*BackupManifest, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	var manifest *BackupManifest
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if manifest == nil {
			if header.Name != "manifest.json" {
				return nil, fmt.Errorf("archive did not start with manifest.json")
			}
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			manifest = &BackupManifest{}
			if err := json.Unmarshal(data, manifest); err != nil {
				return nil, fmt.Errorf("parsing manifest.json: %w", err)
			}
			if err := checkFreeSpace(dir, manifest.TotalDataBytes); err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600); err != nil {
				return nil, fmt.Errorf("staging manifest: %w", err)
			}
			continue
		}

		if err := stageEntry(dir, header, tr); err != nil {
			return nil, err
		}
	}

	if manifest == nil {
		return nil, fmt.Errorf("archive contained no manifest.json")
	}
	return manifest, nil
}

func stageEntry(dir string, header *tar.Header, r io.Reader) error {
	target := filepath.Join(dir, filepath.FromSlash(header.Name))
	cleanDir := filepath.Clean(dir)
	if target != cleanDir && !strings.HasPrefix(target, cleanDir+string(os.PathSeparator)) {
		return fmt.Errorf("archive entry %q escapes the staging directory", header.Name)
	}

	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeSymlink:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.Symlink(header.Linkname, target)
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(f, r); err != nil {
			return err
		}
		if err := os.Chown(target, header.Uid, header.Gid); err != nil {
			logger.Error("backup: failed to restore file ownership", zap.Error(err), zap.String("path", target))
		}
		return nil
	default:
		// devices, fifos, etc. - not expected in app config/data, skip
		return nil
	}
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n2 := n / unit; n2 >= unit; n2 /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// restoreDataPathsWithOverrides moves each staged data directory to its
// destination - the original absolute path it was archived under by
// default, or wherever the review screen redirected that specific volume
// to (destinations, keyed by the ORIGINAL path, from ImportBackupConfirm).
// Must happen before Install, since that's what makes the bind-mount
// sources exist when docker-compose up runs. os.Rename is a cheap
// same-filesystem metadata move (not a copy), since staging and the live
// data root are on the same host filesystem.
func restoreDataPathsWithOverrides(sessionDir string, originalDataPaths []string, destinations map[string]string) error {
	for _, originalPath := range originalDataPaths {
		staged := filepath.Join(sessionDir, filepath.FromSlash(dataArchivePath(originalPath)))
		if _, err := os.Stat(staged); os.IsNotExist(err) {
			// listed in the manifest but nothing was actually archived
			// under it (e.g. an empty directory, or this app's data was
			// excluded at export time) - nothing to move
			continue
		}
		destination := originalPath
		if d, ok := destinations[originalPath]; ok && d != "" {
			destination = d
		}
		if _, err := os.Lstat(destination); err == nil {
			// Something's already at this destination - most often a
			// host-provided system path (a dbus/docker socket, a device
			// node) that every host already has its own live copy of, so
			// ours is neither needed nor safe to drop on top of it (and
			// os.Rename would just fail with EEXIST/ENOTEMPTY anyway).
			// Leave it alone rather than failing this app's entire import
			// over one path.
			logger.Info("backup: restore destination already exists, leaving it in place", zap.String("path", destination))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		if err := os.Rename(staged, destination); err != nil {
			return err
		}
	}
	return nil
}
