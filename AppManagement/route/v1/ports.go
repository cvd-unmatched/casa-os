package v1

import (
	"net/http"

	"github.com/IceWhaleTech/CasaOS-AppManagement/service"
	modelCommon "github.com/IceWhaleTech/CasaOS-Common/model"
	"github.com/IceWhaleTech/CasaOS-Common/utils/common_err"
	"github.com/labstack/echo/v4"
)

// @Summary list every published port across every installed compose app, and which app/service publishes it
// @Produce  application/json
// @Tags ports
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /ports [get]
func ListPortUsage(ctx echo.Context) error {
	usage, err := service.ListPortUsage(ctx.Request().Context())
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, modelCommon.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	return ctx.JSON(http.StatusOK, modelCommon.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: usage})
}
