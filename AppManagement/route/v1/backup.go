package v1

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/IceWhaleTech/CasaOS-AppManagement/service"
	modelCommon "github.com/IceWhaleTech/CasaOS-Common/model"
	"github.com/IceWhaleTech/CasaOS-Common/utils/common_err"
	"github.com/labstack/echo/v4"
)

// @Summary list every exportable app - name, data paths, and data size - without archiving anything
// @Produce  application/json
// @Tags backup
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /backup/apps [get]
func BackupApps(ctx echo.Context) error {
	apps, err := service.ListBackupApps(ctx.Request().Context())
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, modelCommon.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	return ctx.JSON(http.StatusOK, modelCommon.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: apps})
}

// @Summary stream a tar.gz of every managed app's compose config, bind-mounted data, and shared config
// @Produce  application/gzip
// @Tags backup
// @Param exclude_data query string false "comma-separated app names whose data should be skipped (compose config is still included)"
// @Param user_custom query string false "opaque JSON blob (folder groupings, dashboard order) the frontend fetched from elsewhere - embedded as-is, never interpreted here"
// @Security ApiKeyAuth
// @Success 200 {file} file "tar.gz archive"
// @Router /backup/export [get]
func BackupExport(ctx echo.Context) error {
	filename := fmt.Sprintf("casaos-backup-%s.tar.gz", time.Now().UTC().Format("20060102-150405"))

	excludeData := map[string]bool{}
	if raw := ctx.QueryParam("exclude_data"); raw != "" {
		for _, name := range strings.Split(raw, ",") {
			if name = strings.TrimSpace(name); name != "" {
				excludeData[name] = true
			}
		}
	}

	var userCustom []byte
	if raw := ctx.QueryParam("user_custom"); raw != "" {
		if json.Valid([]byte(raw)) {
			userCustom = []byte(raw)
		} else {
			ctx.Logger().Error("backup: ignoring invalid user_custom query param")
		}
	}

	res := ctx.Response()
	res.Header().Set(echo.HeaderContentType, "application/gzip")
	res.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	res.WriteHeader(http.StatusOK)

	if err := service.ExportBackup(ctx.Request().Context(), res, excludeData, userCustom); err != nil {
		// headers (and likely some body) are already flushed by the time a
		// streaming export can fail - nothing left to do but log server-side,
		// there's no clean way to report an error mid-download.
		ctx.Logger().Error(err)
	}
	return nil
}

// @Summary stage an uploaded tar.gz and report what it contains - name/port conflicts, volumes, ports - without installing anything
// @Accept application/octet-stream
// @Produce  application/json
// @Tags backup
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /backup/import/preview [post]
func BackupImportPreview(ctx echo.Context) error {
	preview, err := service.ImportBackupPreview(context.Background(), ctx.Request().Body)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, modelCommon.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, modelCommon.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: preview})
}

type backupImportConfirmRequest struct {
	PreviewID string                       `json:"preview_id"`
	Apps      []service.AppImportDecision  `json:"apps"`
}

// @Summary apply a previously previewed import, with any port/volume edits from the review screen
// @Accept application/json
// @Produce  application/json
// @Tags backup
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /backup/import/confirm [post]
func BackupImportConfirm(ctx echo.Context) error {
	req := backupImportConfirmRequest{}
	if err := (&echo.DefaultBinder{}).BindBody(ctx, &req); err != nil {
		return ctx.JSON(http.StatusBadRequest, modelCommon.Result{Success: common_err.INVALID_PARAMS, Message: err.Error()})
	}
	if req.PreviewID == "" {
		return ctx.JSON(http.StatusBadRequest, modelCommon.Result{Success: common_err.INVALID_PARAMS, Message: "preview_id is required"})
	}

	result, err := service.ImportBackupConfirm(context.Background(), req.PreviewID, req.Apps)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, modelCommon.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, modelCommon.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: result})
}
