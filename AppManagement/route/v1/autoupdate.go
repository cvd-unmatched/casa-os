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
		Data:    service.GetAutoUpdateStatus(),
	})
}

type setAutoUpdatePolicyRequest struct {
	Policy autoupdate.Policy `json:"policy"`
}

// @Summary set an app's auto-update policy (auto/notify/off)
// @Produce  application/json
// @Accept application/json
// @Tags autoupdate
// @Param  name path string true "app name"
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /autoupdate/apps/{name}/policy [put]
func SetAutoUpdatePolicy(ctx echo.Context) error {
	name := ctx.Param("name")
	if name == "" {
		return ctx.JSON(http.StatusBadRequest, modelCommon.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}

	req := setAutoUpdatePolicyRequest{}
	if err := (&echo.DefaultBinder{}).BindBody(ctx, &req); err != nil {
		return ctx.JSON(http.StatusBadRequest, modelCommon.Result{Success: common_err.INVALID_PARAMS, Message: err.Error()})
	}

	switch req.Policy {
	case autoupdate.PolicyAuto, autoupdate.PolicyNotify, autoupdate.PolicyOff:
	default:
		return ctx.JSON(http.StatusBadRequest, modelCommon.Result{Success: common_err.INVALID_PARAMS, Message: "policy must be one of: auto, notify, off"})
	}

	cfg, err := autoupdate.Load()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, modelCommon.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	if cfg.AppPolicies == nil {
		cfg.AppPolicies = map[string]autoupdate.Policy{}
	}
	cfg.AppPolicies[name] = req.Policy

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
