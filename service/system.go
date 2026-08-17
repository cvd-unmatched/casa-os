package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	net2 "net"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/command"
	exec2 "github.com/IceWhaleTech/CasaOS-Common/utils/exec"

	"github.com/IceWhaleTech/CasaOS-Common/utils/file"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/IceWhaleTech/CasaOS/common"
	"github.com/IceWhaleTech/CasaOS/model"
	"github.com/IceWhaleTech/CasaOS/pkg/config"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/common_err"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/httper"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/ip_helper"
	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
	_ "golang.org/x/image/webp"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type SystemService interface {
	UpdateSystemVersion(version string)
	UpdateFromRepo()
	CheckForkUpdate() (needUpdate bool, current string, latest string, releaseNotes string)
	GetSystemConfigDebug() []string
	GetCasaOSLogs(lineNumber int) string
	UpdateAssist()
	UpSystemPort(port string)
	GetTimeZone() string
	UpAppOrderFile(str, id string)
	GetAppOrderFile(id string) []byte
	GetNet(physics bool) []string
	GetNetInfo() []net.IOCountersStat
	GetCpuCoreNum() int
	GetCpuPercent() float64
	GetMemInfo() map[string]interface{}
	GetCpuInfo() []cpu.InfoStat
	GetDirPath(path string) ([]model.Path, error)
	GetDirPathOne(path string) (m model.Path)
	GetNetState(name string) string
	GetDiskInfo() *disk.UsageStat
	GetAllDisksUsage() []DiskUsageInfo
	SaveCustomIcon(mountpoint string, fileHeader *multipart.FileHeader) (string, error)
	SaveCustomIconFromURL(mountpoint string, sourceURL string) (string, error)
	ResolveCustomIconPath(requestedPath string) (string, error)
	GetSysInfo() host.InfoStat
	GetDeviceTree() string
	GetDeviceInfo() model.DeviceInfo
	CreateFile(path string) (int, error)
	RenameFile(oldF, newF string) (int, error)
	MkdirAll(path string) (int, error)
	GetCPUTemperature() int
	GetCPUPower() map[string]string
	GetMacAddress() (string, error)
	SystemReboot() error
	SystemShutdown() error
	GetSystemEntry() string
	GenreateSystemEntry()
}
type systemService struct{}

func (c *systemService) GetDeviceInfo() model.DeviceInfo {
	m := model.DeviceInfo{}
	m.OS_Version = common.VERSION
	err, portStr := MyService.Gateway().GetPort()
	if err != nil {
		m.Port = 80
	} else {
		port := gjson.Get(portStr, "data")
		if len(port.Raw) == 0 {
			m.Port = 80
		} else {
			p, err := strconv.Atoi(port.Raw)
			if err != nil {
				m.Port = 80
			} else {
				m.Port = p
			}
		}
	}
	allIpv4 := ip_helper.GetDeviceAllIPv4()
	ip := []string{}
	nets := MyService.System().GetNet(true)
	for _, n := range nets {
		if v, ok := allIpv4[n]; ok {
			{
				ip = append(ip, v)
			}
		}
	}

	m.LanIpv4 = ip
	h, err := host.Info() /*  */
	if err == nil {
		m.DeviceName = h.Hostname
	}
	mb := model.BaseInfo{}

	err = json.Unmarshal(file.ReadFullFile(config.AppInfo.DBPath+"/baseinfo.conf"), &mb)
	if err == nil {
		m.Hash = mb.Hash
	}

	osRelease, _ := file.ReadOSRelease()
	m.DeviceModel = osRelease["MODEL"]
	m.DeviceSN = osRelease["SN"]
	res := httper.Get("http://127.0.0.1:"+strconv.Itoa(m.Port)+"/v1/users/status", nil)
	init := gjson.Get(res, "data.initialized")
	m.Initialized, _ = strconv.ParseBool(init.Raw)

	return m
}

func (c *systemService) GenreateSystemEntry() {
	modelsPath := "/var/lib/casaos/www/modules"
	entryFileName := "entry.json"
	entryFilePath := filepath.Join(config.AppInfo.DBPath, "db", entryFileName)
	file.IsNotExistCreateFile(entryFilePath)

	dir, err := os.ReadDir(modelsPath)
	if err != nil {
		logger.Error("read dir error", zap.Error(err))
		return
	}
	json := "["
	for _, v := range dir {
		data, err := os.ReadFile(filepath.Join(modelsPath, v.Name(), entryFileName))
		if err != nil {
			logger.Error("read entry file error", zap.Error(err))
			continue
		}
		json += string(data) + ","
	}
	json = strings.TrimRight(json, ",")
	json += "]"
	err = os.WriteFile(entryFilePath, []byte(json), 0o666)
	if err != nil {
		logger.Error("write entry file error", zap.Error(err))
		return
	}
}

func (c *systemService) GetSystemEntry() string {
	modelsPath := "/var/lib/casaos/www/modules"
	entryFileName := "entry.json"
	dir, err := os.ReadDir(modelsPath)
	if err != nil {
		logger.Error("read dir error", zap.Error(err))
		return ""
	}
	json := "["
	for _, v := range dir {
		data, err := os.ReadFile(filepath.Join(modelsPath, v.Name(), entryFileName))
		if err != nil {
			logger.Error("read entry file error", zap.Error(err))
			continue
		}
		json += string(data) + ","
	}
	json = strings.TrimRight(json, ",")
	json += "]"
	if err != nil {
		logger.Error("write entry file error", zap.Error(err))
		return ""
	}
	return json
}

func (c *systemService) GetMacAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	nets := MyService.System().GetNet(true)
	for _, v := range interfaces {
		for _, n := range nets {
			if v.Name == n {
				return v.HardwareAddr, nil
			}
		}
	}
	return "", errors.New("not found")
}

func (c *systemService) MkdirAll(path string) (int, error) {
	_, err := os.Stat(path)
	if err == nil {
		return common_err.DIR_ALREADY_EXISTS, nil
	} else {
		if os.IsNotExist(err) {
			os.MkdirAll(path, os.ModePerm)
			return common_err.SUCCESS, nil
		} else if strings.Contains(err.Error(), ": not a directory") {
			return common_err.FILE_OR_DIR_EXISTS, err
		}
	}
	return common_err.SERVICE_ERROR, err
}

func (c *systemService) RenameFile(oldF, newF string) (int, error) {
	_, err := os.Stat(newF)
	if err == nil {
		return common_err.DIR_ALREADY_EXISTS, nil
	} else {
		if os.IsNotExist(err) {
			err := os.Rename(oldF, newF)
			if err != nil {
				return common_err.SERVICE_ERROR, err
			}
			return common_err.SUCCESS, nil
		}
	}
	return common_err.SERVICE_ERROR, err
}

func (c *systemService) CreateFile(path string) (int, error) {
	_, err := os.Stat(path)
	if err == nil {
		return common_err.FILE_OR_DIR_EXISTS, nil
	} else {
		if os.IsNotExist(err) {
			file.CreateFile(path)
			return common_err.SUCCESS, nil
		}
	}
	return common_err.SERVICE_ERROR, err
}

func (c *systemService) GetDeviceTree() string {
	if output, err := command.OnlyExec("source " + config.AppInfo.ShellPath + "/helper.sh ;GetDeviceTree"); err != nil {
		return ""
	} else {
		return output
	}
}

func (c *systemService) GetSysInfo() host.InfoStat {
	info, _ := host.Info()
	return *info
}

func (c *systemService) GetDiskInfo() *disk.UsageStat {
	path := "/"
	if runtime.GOOS == "windows" {
		path = "C:"
	}
	diskInfo, _ := disk.Usage(path)
	diskInfo.UsedPercent, _ = strconv.ParseFloat(fmt.Sprintf("%.1f", diskInfo.UsedPercent), 64)
	diskInfo.InodesUsedPercent, _ = strconv.ParseFloat(fmt.Sprintf("%.1f", diskInfo.InodesUsedPercent), 64)
	return diskInfo
}

type DiskUsageInfo struct {
	Device      string  `json:"device"`
	Mountpoint  string  `json:"mountpoint"`
	Fstype      string  `json:"fstype"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"usedPercent"`
}

// pseudoFstypes are virtual/in-memory filesystems that show up in `mount`
// output but aren't real storage - not useful in a disk-usage widget.
var pseudoFstypes = map[string]bool{
	"tmpfs": true, "devtmpfs": true, "proc": true, "sysfs": true,
	"cgroup": true, "cgroup2": true, "overlay": true, "aufs": true, "squashfs": true,
	"efivarfs": true, "devpts": true, "securityfs": true, "pstore": true,
	"debugfs": true, "tracefs": true, "mqueue": true, "hugetlbfs": true,
	"configfs": true, "fusectl": true, "bpf": true, "autofs": true,
	"binfmt_misc": true, "rpc_pipefs": true, "nsfs": true,
	"rootfs": true, "ramfs": true, "shm": true, "overlayfs": true,
}

// GetAllDisksUsage lists every real mounted filesystem with its usage, i.e.
// the equivalent of `df -h` (unlike GetDiskInfo, which is scoped to just the
// root filesystem).
func (c *systemService) GetAllDisksUsage() []DiskUsageInfo {
	result := []DiskUsageInfo{}

	// all=true: disk.Partitions(false) filters against /proc/filesystems,
	// which can silently drop real mounted disks depending on which
	// filesystem drivers happen to be loaded. Fetch everything and rely on
	// pseudoFstypes below instead - that also has to catch Docker's
	// per-container overlay mounts, so it needs to be reasonably complete.
	partitions, err := disk.Partitions(true)
	if err != nil {
		return result
	}

	seen := map[string]bool{}
	for _, p := range partitions {
		if pseudoFstypes[p.Fstype] || seen[p.Mountpoint] {
			continue
		}
		seen[p.Mountpoint] = true

		usage, err := disk.Usage(p.Mountpoint)
		if err != nil || usage.Total == 0 {
			continue
		}

		result = append(result, DiskUsageInfo{
			Device:      p.Device,
			Mountpoint:  p.Mountpoint,
			Fstype:      p.Fstype,
			Total:       usage.Total,
			Used:        usage.Used,
			Free:        usage.Free,
			UsedPercent: usage.UsedPercent,
		})
	}

	return result
}

// customIconSubdir is the fixed folder name created under whichever disk the
// user picks for custom app icons - GetCustomIcon (the unauthenticated serve
// route) only ever serves files whose parent directory is exactly this, one
// level under a currently-mounted real disk, so it can't be tricked into
// serving arbitrary files elsewhere on the system.
const customIconSubdir = "casaos-custom-icons"

var allowedIconExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true, ".ico": true,
}

// isMountedDisk checks that mountpoint is one of the currently mounted real
// (non-pseudo) filesystems - used both when saving an icon (don't let the
// frontend point us at an arbitrary directory) and when serving one (don't
// keep serving from a disk that's since been unmounted).
func (c *systemService) isMountedDisk(mountpoint string) bool {
	partitions, err := disk.Partitions(true)
	if err != nil {
		return false
	}
	clean := filepath.Clean(mountpoint)
	for _, p := range partitions {
		if pseudoFstypes[p.Fstype] {
			continue
		}
		if filepath.Clean(p.Mountpoint) == clean {
			return true
		}
	}
	return false
}

// maxIconDimension is the max width/height a saved icon is resized to -
// imaging.Fit only ever scales down, so a smaller source image is left as-is.
const maxIconDimension = 256

// resizeAndEncodeWebP decodes any supported raster format (PNG/JPEG/GIF/WebP),
// downscales it to fit maxIconDimension, and re-encodes it as WebP. This is
// what keeps custom icons small and fast to load regardless of what the
// original upload looked like.
func resizeAndEncodeWebP(src io.Reader) ([]byte, error) {
	img, _, err := image.Decode(src)
	if err != nil {
		return nil, fmt.Errorf("could not decode image: %w", err)
	}

	resized := imaging.Fit(img, maxIconDimension, maxIconDimension, imaging.Lanczos)

	var buf bytes.Buffer
	if err := webp.Encode(&buf, resized, &webp.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("could not encode webp: %w", err)
	}

	return buf.Bytes(), nil
}

func (c *systemService) SaveCustomIcon(mountpoint string, fileHeader *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowedIconExt[ext] {
		return "", fmt.Errorf("unsupported icon file type: %s", ext)
	}

	const maxIconSize = 5 * 1024 * 1024 // 5MB
	if fileHeader.Size > maxIconSize {
		return "", fmt.Errorf("icon file too large (max 5MB)")
	}

	src, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	return c.saveIconFromReader(mountpoint, ext, src)
}

// maxIconDownloadSize caps how much of a remote icon URL is read into memory
// - used by the bulk "convert existing icons to local WebP" feature, which
// has the server (not the browser) fetch each app's current icon URL.
const maxIconDownloadSize = 10 * 1024 * 1024 // 10MB

// SaveCustomIconFromURL downloads sourceURL and saves it the same way an
// uploaded icon file would be saved (resized + re-encoded to WebP, except
// SVG which is kept as-is).
func (c *systemService) SaveCustomIconFromURL(mountpoint string, sourceURL string) (string, error) {
	parsed, err := neturl.Parse(sourceURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("icon url must be http or https")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(sourceURL)
	if err != nil {
		return "", fmt.Errorf("could not fetch icon url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching icon url returned status %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxIconDownloadSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("could not read icon url response: %w", err)
	}
	if len(data) > maxIconDownloadSize {
		return "", fmt.Errorf("icon file too large (max %dMB)", maxIconDownloadSize/1024/1024)
	}

	ext := strings.ToLower(filepath.Ext(parsed.Path))
	if !allowedIconExt[ext] {
		// fall back to sniffing the content, most icon URLs (e.g. dockerhub/
		// icon.casaos.io) don't carry a recognizable extension in the path
		contentType := http.DetectContentType(data)
		switch {
		case strings.Contains(contentType, "svg"):
			ext = ".svg"
		case strings.Contains(contentType, "png"):
			ext = ".png"
		case strings.Contains(contentType, "gif"):
			ext = ".gif"
		case strings.Contains(contentType, "webp"):
			ext = ".webp"
		case strings.Contains(contentType, "jpeg"):
			ext = ".jpg"
		default:
			return "", fmt.Errorf("unsupported icon content type: %s", contentType)
		}
	}

	return c.saveIconFromReader(mountpoint, ext, bytes.NewReader(data))
}

// saveIconFromReader is the shared save path for both an uploaded icon file
// and a downloaded icon URL: validate the disk, resize+re-encode to WebP
// (SVG kept as-is since it's already small and vector), and write it under
// customIconSubdir on the chosen disk.
func (c *systemService) saveIconFromReader(mountpoint string, ext string, src io.Reader) (string, error) {
	if !c.isMountedDisk(mountpoint) {
		return "", fmt.Errorf("%s is not a currently mounted disk", mountpoint)
	}

	if !allowedIconExt[ext] {
		return "", fmt.Errorf("unsupported icon file type: %s", ext)
	}

	dir := filepath.Join(mountpoint, customIconSubdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	if ext == ".svg" {
		destPath := filepath.Join(dir, uuid.NewString()+ext)
		dest, err := os.Create(destPath)
		if err != nil {
			return "", err
		}
		defer dest.Close()

		if _, err := io.Copy(dest, src); err != nil {
			os.Remove(destPath)
			return "", err
		}

		return destPath, nil
	}

	webpData, err := resizeAndEncodeWebP(src)
	if err != nil {
		return "", err
	}

	destPath := filepath.Join(dir, uuid.NewString()+".webp")
	if err := os.WriteFile(destPath, webpData, 0o644); err != nil {
		return "", err
	}

	return destPath, nil
}

// ResolveCustomIconPath validates a requested icon path is actually
// <some currently-mounted disk>/casaos-custom-icons/<filename> before
// GetCustomIcon serves it - this is what keeps the unauthenticated serve
// route from being usable to read arbitrary files off the server.
func (c *systemService) ResolveCustomIconPath(requestedPath string) (string, error) {
	clean := filepath.Clean(requestedPath)
	if !filepath.IsAbs(clean) {
		return "", fmt.Errorf("not an absolute path")
	}

	parentDir := filepath.Dir(clean)
	if filepath.Base(parentDir) != customIconSubdir {
		return "", fmt.Errorf("not under a %s directory", customIconSubdir)
	}

	mountpoint := filepath.Dir(parentDir)
	if !c.isMountedDisk(mountpoint) {
		return "", fmt.Errorf("%s is not a currently mounted disk", mountpoint)
	}

	if info, err := os.Stat(clean); err != nil || info.IsDir() {
		return "", fmt.Errorf("icon file not found")
	}

	return clean, nil
}

func (c *systemService) GetNetState(name string) string {
	if output, err := command.OnlyExec("source " + config.AppInfo.ShellPath + "/helper.sh ;CatNetCardState " + name); err != nil {
		return ""
	} else {
		return output
	}
}

func (c *systemService) GetDirPathOne(path string) (m model.Path) {
	f, err := os.Stat(path)
	if err != nil {
		return
	}
	m.IsDir = f.IsDir()
	m.Name = f.Name()
	m.Path = path
	m.Size = f.Size()
	m.Date = f.ModTime()
	return
}

func (c *systemService) GetDirPath(path string) ([]model.Path, error) {
	if path == "/DATA" {
		sysType := runtime.GOOS
		if sysType == "windows" {
			path = "C:\\CasaOS\\DATA"
		}
		if sysType == "darwin" {
			path = "./CasaOS/DATA"
		}

	}

	ls, err := os.ReadDir(path)
	if err != nil {
		logger.Error("when read dir", zap.Error(err))
		return []model.Path{}, err
	}
	dirs := []model.Path{}
	if len(path) > 0 {
		for _, l := range ls {
			filePath := filepath.Join(path, l.Name())
			link, err := filepath.EvalSymlinks(filePath)
			if err != nil {
				link = filePath
			}
			tempFile, err := l.Info()
			if err != nil {
				logger.Error("when read dir", zap.Error(err))
				return []model.Path{}, err
			}
			temp := model.Path{Name: l.Name(), Path: filePath, IsDir: l.IsDir(), Date: tempFile.ModTime(), Size: tempFile.Size()}
			if filePath != link {
				file, _ := os.Stat(link)
				temp.IsDir = file.IsDir()
			}
			dirs = append(dirs, temp)
		}
	} else {
		dirs = append(dirs, model.Path{Name: "DATA", Path: "/DATA/", IsDir: true, Date: time.Now()})
	}
	return dirs, nil
}

func (c *systemService) GetCpuInfo() []cpu.InfoStat {
	info, _ := cpu.Info()
	return info
}

func (c *systemService) GetMemInfo() map[string]interface{} {
	memInfo, _ := mem.VirtualMemory()
	memInfo.UsedPercent, _ = strconv.ParseFloat(fmt.Sprintf("%.1f", memInfo.UsedPercent), 64)
	memData := make(map[string]interface{})
	memData["total"] = memInfo.Total
	memData["available"] = memInfo.Available
	memData["used"] = memInfo.Used
	memData["free"] = memInfo.Free
	memData["usedPercent"] = memInfo.UsedPercent
	return memData
}

func (c *systemService) GetCpuPercent() float64 {
	percent, _ := cpu.Percent(0, false)
	value, _ := strconv.ParseFloat(fmt.Sprintf("%.1f", percent[0]), 64)
	return value
}

func (c *systemService) GetCpuCoreNum() int {
	count, _ := cpu.Counts(false)
	return count
}

func (c *systemService) GetNetInfo() []net.IOCountersStat {
	parts, _ := net.IOCounters(true)
	return parts
}

func (c *systemService) GetNet(physics bool) []string {
	t := "1"
	if physics {
		t = "2"
	}

	if output, err := command.OnlyExec("source " + config.AppInfo.ShellPath + "/helper.sh ;GetNetCard " + t); err != nil {
		return []string{}
	} else {
		return strings.Split(output, "\n")
	}
}

func (s *systemService) UpdateSystemVersion(version string) {
	keyName := "casa_version"
	Cache.Delete(keyName)
	if file.Exists(config.AppInfo.LogPath + "/upgrade.log") {
		os.Remove(config.AppInfo.LogPath + "/upgrade.log")
	}
	file.CreateFile(config.AppInfo.LogPath + "/upgrade.log")
	// go command2.OnlyExec("curl -fsSL https://raw.githubusercontent.com/LinkLeong/casaos-alpha/main/update.sh | bash")
	if len(config.ServerInfo.UpdateUrl) > 0 {
		go command.OnlyExec("curl -fsSL " + config.ServerInfo.UpdateUrl + " | bash")
	} else {
		osRelease, _ := file.ReadOSRelease()
		go command.OnlyExec("curl -fsSL https://get.casaos.io/update?t=" + osRelease["MANUFACTURER"] + " | bash")
	}

	// s.log.Error(config.AppInfo.ProjectPath + "/shell/tool.sh -r " + version)
	// s.log.Error(command2.ExecResultStr(config.AppInfo.ProjectPath + "/shell/tool.sh -r " + version))
}

// UpdateFromRepo pulls the latest release of this fork (github.com/cvd-unmatched/casa-os)
// and swaps it in, via the repo's own update.sh (same script documented in FORK.md for
// manual use). Runs detached, same as UpdateSystemVersion above, since the update script
// stops this very process partway through - the HTTP handler must return before that
// happens, and the frontend polls for the service coming back afterwards.
func (s *systemService) UpdateFromRepo() {
	// update.sh itself runs `systemctl stop casaos` partway through. Spawned
	// as a plain child of this process, that would kill update.sh along
	// with it - systemd's default KillMode=control-group tears down the
	// *entire* cgroup of a stopped service, including any child process
	// still running inside it, regardless of process-group/session
	// detachment. Running it via `systemd-run` gives it its own independent
	// transient unit/cgroup, so stopping casaos doesn't take the updater
	// down with it.
	go command.OnlyExec("systemd-run --unit=casaos-fork-update --collect /bin/bash -c 'curl -fsSL https://raw.githubusercontent.com/cvd-unmatched/casa-os/main/update.sh | bash'")
}

// GitHub's unauthenticated API allows 60 requests/hour *per source IP* -
// hitting this on every dashboard load/reload burns through that fast (and
// on failure, everything below silently returned "no update" with no way to
// tell that apart from a genuine up-to-date check - see the cache and the
// explicit error-response check below). 15 minutes keeps this well within
// budget even with frequent reloads, without making a real new release take
// unreasonably long to show up.
const forkUpdateCacheTTL = 15 * time.Minute

var (
	forkUpdateCacheMu     sync.Mutex
	forkUpdateCacheAt     time.Time
	forkUpdateCacheNeed   bool
	forkUpdateCacheLatest string
	forkUpdateCacheNotes  string
)

// CheckForkUpdate compares the release tag this binary was built from
// (common.ForkVersion, set via -ldflags in .github/workflows/release.yml)
// against the latest tag published on this fork's own GitHub repo - not
// IceWhale's api.casaos.io. Unlike UpdateSystemVersion's check, a binary
// built any other way than that release workflow (e.g. a local `go build`)
// has an empty ForkVersion, in which case we can't tell and say no update
// is needed rather than guessing.
//
// A empty latest return value (with needUpdate false) means the check
// itself didn't complete - rate limited, network error, etc. - and is
// deliberately distinguishable from "checked, genuinely up to date" so
// callers don't report a false "up to date" on a failed check.
func (s *systemService) CheckForkUpdate() (needUpdate bool, current string, latest string, releaseNotes string) {
	current = common.ForkVersion
	if current == "" {
		return false, current, "", ""
	}

	forkUpdateCacheMu.Lock()
	if time.Since(forkUpdateCacheAt) < forkUpdateCacheTTL {
		needUpdate, latest, releaseNotes = forkUpdateCacheNeed, forkUpdateCacheLatest, forkUpdateCacheNotes
		forkUpdateCacheMu.Unlock()
		return needUpdate, current, latest, releaseNotes
	}
	forkUpdateCacheMu.Unlock()

	// fetch a page of releases (not just /releases/latest) so that updating
	// across several versions at once (e.g. v1.0.31 -> v1.0.34) can show
	// everything that changed across all of them, not just the latest tag's
	// own notes - GitHub returns these newest-first.
	resp := httper.Get("https://api.github.com/repos/cvd-unmatched/casa-os/releases?per_page=30", map[string]string{
		"Accept":     "application/vnd.github+json",
		"User-Agent": "casaos-fork-update-check",
	})
	parsed := gjson.Parse(resp)

	// GitHub error responses (rate limited, not found, etc.) are a JSON
	// object with a "message" field, not an array of releases - .Array()
	// would silently return empty for these too, which is indistinguishable
	// from "genuinely no releases", so check for this explicitly and log it
	// rather than let a rate-limit failure quietly look like "up to date".
	if message := parsed.Get("message").String(); message != "" {
		logger.Error("failed to check fork releases", zap.String("message", message))
		cacheForkUpdateResult(false, "", "")
		return false, current, "", ""
	}

	releases := parsed.Array()
	if len(releases) == 0 {
		cacheForkUpdateResult(false, "", "")
		return false, current, "", ""
	}

	latest = releases[0].Get("tag_name").String()
	if latest == "" {
		cacheForkUpdateResult(false, "", "")
		return false, current, "", ""
	}

	// release notes are auto-generated by the release workflow (generate_release_notes: true
	// in .github/workflows/release.yml) from the commits since each release's previous tag, so
	// this is always current with whatever actually shipped - nothing to maintain by hand. Walk
	// newest to oldest, collecting every release's notes until reaching the one currently running.
	var notes strings.Builder
	for _, release := range releases {
		if release.Get("tag_name").String() == current {
			break
		}
		if notes.Len() > 0 {
			notes.WriteString("\n\n")
		}
		notes.WriteString(release.Get("body").String())
	}
	releaseNotes = notes.String()
	needUpdate = latest != current

	cacheForkUpdateResult(needUpdate, latest, releaseNotes)

	return needUpdate, current, latest, releaseNotes
}

func cacheForkUpdateResult(needUpdate bool, latest, releaseNotes string) {
	forkUpdateCacheMu.Lock()
	defer forkUpdateCacheMu.Unlock()
	forkUpdateCacheAt = time.Now()
	forkUpdateCacheNeed = needUpdate
	forkUpdateCacheLatest = latest
	forkUpdateCacheNotes = releaseNotes
}

func (s *systemService) UpdateAssist() {
	command.ExecResultStrArray("source " + config.AppInfo.ShellPath + "/assist.sh")
}

func (s *systemService) GetTimeZone() string {
	if output, err := command.OnlyExec("source " + config.AppInfo.ShellPath + "/helper.sh ;GetTimeZone"); err != nil {
		return ""
	} else {
		return output
	}
}

func (s *systemService) GetSystemConfigDebug() []string {
	if output, err := command.OnlyExec("source " + config.AppInfo.ShellPath + "/helper.sh ;GetSysInfo"); err != nil {
		return []string{}
	} else {
		return strings.Split(output, "\n")
	}
}

func (s *systemService) UpAppOrderFile(str, id string) {
	file.WriteToPath([]byte(str), config.AppInfo.DBPath+"/"+id, "app_order.json")
}

func (s *systemService) GetAppOrderFile(id string) []byte {
	return file.ReadFullFile(config.AppInfo.UserDataPath + "/" + id + "/app_order.json")
}

func (s *systemService) UpSystemPort(port string) {
	if len(port) > 0 && port != config.ServerInfo.HttpPort {
		config.Cfg.Section("server").Key("HttpPort").SetValue(port)
		config.ServerInfo.HttpPort = port
	}
	config.Cfg.SaveTo(config.SystemConfigInfo.ConfigPath)
}

func (s *systemService) GetCasaOSLogs(lineNumber int) string {
	file, err := os.Open(filepath.Join(config.AppInfo.LogPath, fmt.Sprintf("%s.%s",
		config.AppInfo.LogSaveName,
		config.AppInfo.LogFileExt,
	)))
	if err != nil {
		return err.Error()
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return err.Error()
	}

	return string(content)
}

func GetDeviceAllIP() []string {
	var address []string
	addrs, err := net2.InterfaceAddrs()
	if err != nil {
		return address
	}
	for _, a := range addrs {
		if ipNet, ok := a.(*net2.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To16() != nil {
				address = append(address, ipNet.IP.String())
			}
		}
	}
	return address
}

// find thermal_zone of cpu.
// assertions:
//   - thermal_zone "type" and "temp" are required fields
//     (https://www.kernel.org/doc/Documentation/ABI/testing/sysfs-class-thermal)
func GetCPUThermalZone() string {
	keyName := "cpu_thermal_zone"

	var path string
	if result, ok := Cache.Get(keyName); ok {
		path, ok = result.(string)
		if ok {
			return path
		}
	}

	var name string
	cpu_types := []string{"x86_pkg_temp", "cpu", "CPU", "soc"}
	stub := "/sys/devices/virtual/thermal/thermal_zone"
	for i := 0; i < 100; i++ {
		path = stub + strconv.Itoa(i)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			name = strings.TrimSuffix(string(file.ReadFullFile(path+"/type")), "\n")
			for _, s := range cpu_types {
				if strings.HasPrefix(name, s) {
					//logger.Info(fmt.Sprintf("CPU thermal zone found: %s, path: %s.", name, path))
					Cache.SetDefault(keyName, path)
					return path
				}
			}
		} else {
			if len(name) > 0 { // proves at least one zone
				path = stub + "0"
			} else {
				path = ""
			}
			break
		}
	}

	Cache.SetDefault(keyName, path)
	return path
}

func (s *systemService) GetCPUTemperature() int {
	outPut := ""
	path := GetCPUThermalZone()
	if len(path) > 0 {
		outPut = string(file.ReadFullFile(path + "/temp"))
	} else {
		outPut = string(file.ReadFullFile("/sys/class/hwmon/hwmon0/temp1_input"))
		if len(outPut) == 0 {
			outPut = "0"
		}
	}

	celsius, _ := strconv.Atoi(strings.TrimSpace(outPut))

	if celsius > 1000 {
		celsius = celsius / 1000
	}
	return celsius
}

func (s *systemService) GetCPUPower() map[string]string {
	data := make(map[string]string, 2)
	data["timestamp"] = strconv.FormatInt(time.Now().Unix(), 10)
	if file.Exists("/sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj") {
		data["value"] = strings.TrimSpace(string(file.ReadFullFile("/sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj")))
	} else {
		data["value"] = "0"
	}
	return data
}

func (s *systemService) SystemReboot() error {
	arg := []string{"6"}
	cmd := exec2.Command("init", arg...)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	return nil
}

func (s *systemService) SystemShutdown() error {
	arg := []string{"0"}
	cmd := exec2.Command("init", arg...)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	return nil
}

func NewSystemService() SystemService {
	return &systemService{}
}
