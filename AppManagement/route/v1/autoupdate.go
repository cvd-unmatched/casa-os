package v1

import (
	"context"
	"net/http"

	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/autoupdate"
	"github.com/IceWhaleTech/CasaOS-AppManagement/service"
	modelCommon "github.com/IceWhaleTech/CasaOS-Common/model"
	"github.com/IceWhaleTech/CasaOS-Common/utils/common_err"
	"github.com/labstack/echo/v4"
)

// @Summary list every CasaOS-managed app's auto-update status and policy
// @Produce  application/json
// @Tags autoupdate
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /autoupdate/apps [get]
func ListAutoUpdateStatus(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, modelCommon.Result{
		Success: common_err.SUCCESS,
		Message: common_err.GetMsg(common_err.SUCCESS),
		Data:    service.GetAutoUpdateStatus(context.Background()),
	})
}

// @Summary set an app's auto-update and notify settings
// @Produce  application/json
// @Accept application/json
// @Tags autoupdate
// @Param  name path string true "app name"
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /autoupdate/apps/{name}/settings [put]
func SetAutoUpdateSettings(ctx echo.Context) error {
	name := ctx.Param("name")
	if name == "" {
		return ctx.JSON(http.StatusBadRequest, modelCommon.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}

	req := autoupdate.AppSettings{}
	if err := (&echo.DefaultBinder{}).BindBody(ctx, &req); err != nil {
		return ctx.JSON(http.StatusBadRequest, modelCommon.Result{Success: common_err.INVALID_PARAMS, Message: err.Error()})
	}

	cfg, err := autoupdate.Load()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, modelCommon.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	if cfg.Apps == nil {
		cfg.Apps = map[string]autoupdate.AppSettings{}
	}
	cfg.Apps[name] = req

	if err := autoupdate.Save(cfg); err != nil {
		return ctx.JSON(http.StatusInternalServerError, modelCommon.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, modelCommon.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

// @Summary force a synchronous, read-only registry check for one app - never applies an update
// @Produce  application/json
// @Tags autoupdate
// @Param  name path string true "app name"
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /autoupdate/apps/{name}/recheck [post]
func RecheckApp(ctx echo.Context) error {
	name := ctx.Param("name")
	if name == "" {
		return ctx.JSON(http.StatusBadRequest, modelCommon.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}

	status, err := service.RecheckApp(context.Background(), name)
	if err != nil {
		return ctx.JSON(http.StatusNotFound, modelCommon.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, modelCommon.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: status})
}

// @Summary check every app now and apply the update for each one with auto-update on
// @Produce  application/json
// @Tags autoupdate
// @Security ApiKeyAuth
// @Success 202 {string} string "started"
// @Failure 409 {string} string "a run is already in progress"
// @Router /autoupdate/run [post]
func RunAutoUpdate(ctx echo.Context) error {
	if !service.StartAutoUpdateRun() {
		return ctx.JSON(http.StatusConflict, modelCommon.Result{Success: common_err.SERVICE_ERROR, Message: "an update run is already in progress"})
	}

	return ctx.JSON(http.StatusAccepted, modelCommon.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

// @Summary whether a check-and-update run is in progress
// @Produce  application/json
// @Tags autoupdate
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /autoupdate/run [get]
func GetAutoUpdateRun(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, modelCommon.Result{
		Success: common_err.SUCCESS,
		Message: common_err.GetMsg(common_err.SUCCESS),
		Data:    map[string]bool{"running": service.AutoUpdateRunning()},
	})
}
