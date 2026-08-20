//go:generate bash -c "mkdir -p codegen && go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.12.4 -generate types,server,spec -package codegen api/app_management/openapi.yaml > codegen/app_management_api.go"
//go:generate bash -c "mkdir -p codegen/message_bus && go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.12.4 -generate types,client -package message_bus api/message_bus/openapi.yaml > codegen/message_bus/api.go"

package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/IceWhaleTech/CasaOS-AppManagement/common"
	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/config"
	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/webhook"
	"github.com/IceWhaleTech/CasaOS-AppManagement/route"
	"github.com/IceWhaleTech/CasaOS-AppManagement/service"
	"github.com/IceWhaleTech/CasaOS-Common/model"
	"github.com/IceWhaleTech/CasaOS-Common/utils/file"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/coreos/go-systemd/daemon"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	util_http "github.com/IceWhaleTech/CasaOS-Common/utils/http"
)

var (
	commit = "private build"
	date   = "private build"

	// forkVersion is this fork's release tag (e.g. "v1.8.7"), injected via
	// -ldflags at build time (see .github/workflows/release.yml's
	// build-app-management job) - mirrors how the root casaos module
	// stamps common.ForkVersion. Stamped onto outbound webhook.Version so
	// notifications from this service show which build sent them.
	forkVersion = "private build"

	//go:embed api/index.html
	_docHTML string

	//go:embed api/index_v1.html
	_docHTMLV1 string

	//go:embed api/app_management/openapi.yaml
	_docYAML string

	//go:embed api/app_management/openapi_v1.yaml
	_docYAMLV1 string

	//go:embed build/sysroot/etc/casaos/app-management.conf.sample
	_confSample string
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// parse arguments and intialize
	{
		configFlag := flag.String("c", "", "config file path")
		versionFlag := flag.Bool("v", false, "version")
		removeRuntimeIfNoNvidiaGPUFlag := flag.Bool("removeRuntimeIfNoNvidiaGPU", false, "remove runtime with nvidia gpu")

		flag.Parse()

		if *versionFlag {
			fmt.Printf("v%s\n", common.AppManagementVersion)
			os.Exit(0)
		}

		println("git commit:", commit)
		println("build date:", date)

		config.InitSetup(*configFlag, _confSample)
		config.InitGlobal(*configFlag)

		logger.LogInit(config.AppInfo.LogPath, config.AppInfo.LogSaveName, config.AppInfo.LogFileExt)

		service.MyService = service.NewService(config.CommonInfo.RuntimePath)

		config.RemoveRuntimeIfNoNvidiaGPUFlag = *removeRuntimeIfNoNvidiaGPUFlag

		webhook.Version = forkVersion
		service.ForkVersion = forkVersion
	}

	// setup cron
	{
		crontab := cron.New(cron.WithSeconds())

		// schedule async v2job to get v2 appstore list
		go func() {
			// run once at startup
			if err := service.MyService.AppStoreManagement().UpdateCatalog(); err != nil {
				logger.Error("error when updating AppStore catalog at startup", zap.Error(err))
			}
		}()

		if _, err := crontab.AddFunc("@every 10m", func() {
			if err := service.MyService.AppStoreManagement().UpdateCatalog(); err != nil {
				logger.Error("error when updating AppStore catalog", zap.Error(err))
			}
		}); err != nil {
			panic(err)
		}

		// auto-updater: checks every CasaOS-managed app's images against
		// their registries for a newer semver tag (not the old catalog-only
		// digest comparison, which silently never checked apps installed
		// outside the app store), notifies via webhook, and applies the
		// update for any app with auto-update enabled (see pkg/autoupdate).
		// The "already notified" state persists to disk (pkg/autoupdate's
		// NotifiedTracker) so a restart doesn't cause the same
		// still-unapplied update to be re-announced.

		// run once at startup, same as the appstore catalog job above -
		// otherwise the status cache (what GET /v1/autoupdate/apps serves)
		// sits empty for up to an hour after every restart, since
		// cron.AddFunc's @every schedule only fires the NEXT occurrence,
		// not immediately.
		//
		// runAutoUpdateCheck recovers from any panic in this new, less-
		// proven code path - an unrecovered panic in a goroutine crashes
		// the ENTIRE process (confirmed live: a bug here was taking down
		// casaos-app-management in a restart loop, breaking everything
		// this service does, not just auto-update). A bug should degrade
		// this one feature for one cycle, not the whole service.
		go runAutoUpdateCheck(ctx)

		if _, err := crontab.AddFunc("@every 1h", func() {
			runAutoUpdateCheck(ctx)
		}); err != nil {
			panic(err)
		}

		// an import preview (POST /v1/backup/import/preview) stages an
		// uploaded archive on disk and leaves it there for a later confirm
		// call - if the user never confirms, nothing else ever cleans that
		// up. An hour is generous for "review and confirm" while still
		// keeping abandoned uploads from accumulating indefinitely.
		if _, err := crontab.AddFunc("@every 20m", func() {
			service.SweepStaleImportPreviews(time.Hour)
		}); err != nil {
			panic(err)
		}

		crontab.Start()
		defer crontab.Stop()

	}

	// webhook notifications: watch for containers crashing
	go service.StartCrashWatcher(ctx)

	// register at message bus
	{
		response, err := service.MyService.MessageBus().RegisterEventTypesWithResponse(ctx, common.EventTypes)
		if err != nil {
			logger.Error("error when trying to register one or more event types - some event type will not be discoverable", zap.Error(err))
		}

		if response != nil && response.StatusCode() != http.StatusOK {
			logger.Error("error when trying to register one or more event types - some event type will not be discoverable", zap.String("status", response.Status()), zap.String("body", string(response.Body)))
		}
	}

	// setup listener
	listener, err := net.Listen("tcp", net.JoinHostPort(common.Localhost, "0"))
	if err != nil {
		panic(err)
	}

	urlFilePath := filepath.Join(config.CommonInfo.RuntimePath, "app-management.url")
	if err := file.CreateFileAndWriteContent(urlFilePath, "http://"+listener.Addr().String()); err != nil {
		logger.Error("error when creating address file", zap.Error(err),
			zap.Any("address", listener.Addr().String()),
			zap.Any("filepath", urlFilePath),
		)
	}

	// initialize routers and register at gateway
	{
		apiPaths := []string{
			"/v1/apps",
			"/v1/container",
			"/v1/app-categories",
			"/v1/autoupdate",
			"/v1/backup",
			route.V1DocPath,
			route.V2APIPath,
			route.V2DocPath,
		}

		for _, apiPath := range apiPaths {
			if err := service.MyService.Gateway().CreateRoute(&model.Route{
				Path:   apiPath,
				Target: "http://" + listener.Addr().String(),
			}); err != nil {
				panic(err)
			}
		}
	}

	v1Router := route.InitV1Router()
	v2Router := route.InitV2Router()
	v1DocRouter := route.InitV1DocRouter(_docHTMLV1, _docYAMLV1)
	v2DocRouter := route.InitV2DocRouter(_docHTML, _docYAML)

	mux := &util_http.HandlerMultiplexer{
		HandlerMap: map[string]http.Handler{
			"v1":    v1Router,
			"v2":    v2Router,
			"v1doc": v1DocRouter,
			"doc":   v2DocRouter,
		},
	}

	// notify systemd that we are ready
	{
		if supported, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
			logger.Error("Failed to notify systemd that casaos main service is ready", zap.Any("error", err))
		} else if supported {
			logger.Info("Notified systemd that casaos main service is ready")
		} else {
			logger.Info("This process is not running as a systemd service.")
		}

		logger.Info("App management service is listening...", zap.Any("address", listener.Addr().String()))
	}

	s := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second, // fix G112: Potential slowloris attack (see https://github.com/securego/gosec)
	}

	err = s.Serve(listener) // not using http.serve() to fix G114: Use of net/http serve function that has no support for setting timeouts (see https://github.com/securego/gosec)
	if err != nil {
		panic(err)
	}
}

// runAutoUpdateCheck runs service.CheckAndApplyAutoUpdates with panic
// recovery - this is called from its own goroutine both at startup and on
// every cron tick, and an unrecovered panic in a goroutine takes down the
// entire process, not just this one check (confirmed live: a bug here put
// casaos-app-management into a restart loop, breaking every feature this
// service provides, not just auto-update, until diagnosed). Logs the full
// stack on recovery so the underlying bug is still visible and fixable.
func runAutoUpdateCheck(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("autoupdate: recovered from panic - see stack below",
				zap.Any("panic", r), zap.String("stack", string(debug.Stack())))
		}
	}()
	service.CheckAndApplyAutoUpdates(ctx)
}

