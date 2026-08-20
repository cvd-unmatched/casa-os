package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/CasaOS-AppManagement/common"
	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/autoupdate"
	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/webhook"
	v1 "github.com/IceWhaleTech/CasaOS-AppManagement/service/v1"
	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
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
	sourceType  string
	composeYAML []byte
	dataPaths   []string
}

// ExportBackup streams every managed app's compose config, bind-mounted
// data, and the shared webhook/autoupdate config as a single tar.gz written
// directly to w - no buffering of the whole archive in memory or on disk.
func ExportBackup(ctx context.Context, w io.Writer) error {
	apps, err := collectBackupApps(ctx)
	if err != nil {
		return err
	}

	manifest := BackupManifest{
		FormatVersion: backupFormatVersion,
		CreatedAt:     time.Now().UTC(),
		ForkVersion:   ForkVersion,
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

func composeAppToBackupApp(composeApp *ComposeApp) (backupApp, error) {
	yamlBytes, err := GenerateYAMLFromComposeApp(*composeApp)
	if err != nil {
		return backupApp{}, err
	}
	return backupApp{
		name:        composeApp.Name,
		sourceType:  "compose",
		composeYAML: yamlBytes,
		dataPaths:   bindMountSourcesOf(composeApp),
	}, nil
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
			if vol.Type != "bind" || vol.Source == "" || seen[vol.Source] {
				continue
			}
			seen[vol.Source] = true
			paths = append(paths, vol.Source)
		}
	}
	return paths
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

// ImportBackup extracts an archive ExportBackup produced into a fresh
// staging directory first, then installs whatever doesn't collide with
// what's already on this server. A failure at any point before every app
// has been processed leaves already-running apps untouched - the staging
// directory is always removed, success or failure.
func ImportBackup(ctx context.Context, r io.Reader) (*ImportResult, error) {
	sessionDir := filepath.Join(stagingRoot, uuid.NewV4().String())
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating staging directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(sessionDir); err != nil {
			logger.Error("backup: failed to clean up staging directory", zap.Error(err), zap.String("path", sessionDir))
		}
	}()

	manifest, err := extractArchive(r, sessionDir)
	if err != nil {
		return nil, fmt.Errorf("extracting archive: %w", err)
	}

	restoreConfigFiles(sessionDir)

	return applyStagedApps(ctx, sessionDir, manifest), nil
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
// transfer.
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

// applyStagedApps is phase B of import: for every app in the manifest, skip
// on a name collision (checked against both compose and standalone apps -
// they share one namespace) or a port conflict, otherwise move its staged
// data onto its final live path and install it. Each app is independent -
// one failure doesn't block or roll back the rest.
func applyStagedApps(ctx context.Context, sessionDir string, manifest *BackupManifest) *ImportResult {
	result := &ImportResult{}

	existingCompose, err := MyService.Compose().List(ctx)
	if err != nil {
		logger.Error("backup: failed to list existing compose apps for collision check", zap.Error(err))
		existingCompose = map[string]*ComposeApp{}
	}
	existingStandalone := map[string]bool{}
	if casaOSApps, _ := MyService.Docker().GetContainerAppList(nil, nil, nil); casaOSApps != nil {
		for _, app := range *casaOSApps {
			existingStandalone[app.Name] = true
		}
	}

	for _, entry := range manifest.Apps {
		if _, exists := existingCompose[entry.Name]; exists || existingStandalone[entry.Name] {
			result.Skipped = append(result.Skipped, SkippedApp{Name: entry.Name, Reason: "an app with this name already exists on this server"})
			continue
		}

		composePath := filepath.Join(sessionDir, filepath.FromSlash(entry.ComposePath))
		yamlBytes, err := os.ReadFile(composePath)
		if err != nil {
			result.Failed = append(result.Failed, FailedApp{Name: entry.Name, Reason: "missing compose file in archive: " + err.Error()})
			continue
		}

		composeApp, err := NewComposeAppFromYAML(yamlBytes, false, true)
		if err != nil {
			result.Failed = append(result.Failed, FailedApp{Name: entry.Name, Reason: "invalid compose file: " + err.Error()})
			continue
		}

		validation, err := composeApp.GetPortsInUse()
		if err != nil {
			result.Failed = append(result.Failed, FailedApp{Name: entry.Name, Reason: "checking port conflicts: " + err.Error()})
			continue
		}
		if validation != nil {
			result.Skipped = append(result.Skipped, SkippedApp{Name: entry.Name, Reason: "one or more ports are already in use"})
			continue
		}

		if err := restoreDataPaths(sessionDir, entry.DataPaths); err != nil {
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

	return result
}

// restoreDataPaths moves each staged data directory to its final absolute
// host path - must happen before Install, since that's what makes the
// bind-mount sources exist when docker-compose up runs. os.Rename is a
// cheap same-filesystem metadata move (not a copy), since staging and the
// live data root are on the same host filesystem.
func restoreDataPaths(sessionDir string, dataPaths []string) error {
	for _, absPath := range dataPaths {
		staged := filepath.Join(sessionDir, filepath.FromSlash(dataArchivePath(absPath)))
		if _, err := os.Stat(staged); os.IsNotExist(err) {
			// listed in the manifest but nothing was actually archived
			// under it (e.g. an empty directory) - nothing to move
			continue
		}
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return err
		}
		if err := os.Rename(staged, absPath); err != nil {
			return err
		}
	}
	return nil
}
