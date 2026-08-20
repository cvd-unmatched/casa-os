package v1

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/IceWhaleTech/CasaOS-AppManagement/service"
	modelCommon "github.com/IceWhaleTech/CasaOS-Common/model"
	"github.com/IceWhaleTech/CasaOS-Common/utils/common_err"
	"github.com/labstack/echo/v4"
)

// @Summary stream a tar.gz of every managed app's compose config, bind-mounted data, and shared config
// @Produce  application/gzip
// @Tags backup
// @Security ApiKeyAuth
// @Success 200 {file} file "tar.gz archive"
// @Router /backup/export [get]
func BackupExport(ctx echo.Context) error {
	filename := fmt.Sprintf("casaos-backup-%s.tar.gz", time.Now().UTC().Format("20060102-150405"))

	res := ctx.Response()
	res.Header().Set(echo.HeaderContentType, "application/gzip")
	res.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	res.WriteHeader(http.StatusOK)

	if err := service.ExportBackup(ctx.Request().Context(), res); err != nil {
		// headers (and likely some body) are already flushed by the time a
		// streaming export can fail - nothing left to do but log server-side,
		// there's no clean way to report an error mid-download.
		ctx.Logger().Error(err)
	}
	return nil
}

// @Summary restore apps from a tar.gz produced by /backup/export
// @Accept application/octet-stream
// @Produce  application/json
// @Tags backup
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /backup/import [post]
func BackupImport(ctx echo.Context) error {
	result, err := service.ImportBackup(context.Background(), ctx.Request().Body)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, modelCommon.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, modelCommon.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: result})
}
