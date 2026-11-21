package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/patch"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// MeHandler 当前登录用户的处理器（Kubernetes 风格单例资源）。
type MeHandler struct {
	me *services.MeService
}

// NewMeHandler 构造 MeHandler 并注入依赖。
func NewMeHandler(i do.Injector) (*MeHandler, error) {
	return &MeHandler{
		me: do.MustInvoke[*services.MeService](i),
	}, nil
}

// Routes 注册路由。
//
// /api/v1/me 是一个单例资源（singleton resource）—— 路径上没有资源 ID，
// 因为它隐式绑定到当前认证用户。CRUD 集合中只有 Get / Replace (PUT) / Patch (PATCH)。
//
// change-password 作为子资源动作单独暴露：密码字段不属于资料表示，
// 也不能用全量替换语义进行轮换（需要校验旧密码）。
func (h *MeHandler) Routes(g *echo.Group, auth echo.MiddlewareFunc) {
	r := g.Group("/me", auth)

	r.GET("", h.Get)
	r.PUT("", h.Replace)
	r.PATCH("", h.Patch)
	r.POST("/change-password", h.ChangePassword)
}

// toUserInfo 将 *ent.User 映射为对外的资料 DTO（不含密码、删除标记等）。
func toUserInfo(u *ent.User) types.UserInfo {
	return types.UserInfo{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		Status:      string(u.Status),
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
	}
}

// Get 获取当前用户信息
//
//	@Summary	获取当前用户信息
//	@Tags		me
//	@Security	BearerAuth
//	@Produce	json
//	@Success	200	{object}	types.UserInfo
//	@Failure	401	{object}	ErrorResponse
//	@Router		/api/v1/me [get]
func (h *MeHandler) Get(c *echo.Context) error {
	user := appctx.MustGetUserFromContext(c.Request().Context())
	return c.JSON(http.StatusOK, toUserInfo(user))
}

// Replace 替换当前用户资料（幂等全量更新，Kubernetes 风格 PUT）。
//
//	@Summary		替换当前用户资料
//	@Description	使用请求体中的 username 与 email 覆盖现有资料。username / email 各自全局唯一，冲突时返回 409。
//	@Tags			me
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		types.ReplaceMeRequest	true	"完整资料"
//	@Success		200		{object}	types.UserInfo
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse	"username / email 已被占用"
//	@Router			/api/v1/me [put]
func (h *MeHandler) Replace(c *echo.Context) error {
	ctx := c.Request().Context()
	user := appctx.MustGetUserFromContext(ctx)

	var req types.ReplaceMeRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	u, err := h.me.Replace(ctx, user.ID, &req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toUserInfo(u))
}

// Patch 局部更新当前用户资料（多协议 PATCH）。
//
// 根据 Content-Type 选择 patch 协议；统一通过"GET → apply → Replace"流程落库。
//
//	@Summary		局部更新当前用户资料
//	@Description	根据 Content-Type 选择 patch 协议：application/json-patch+json (RFC 6902) / application/merge-patch+json (RFC 7386) / application/merge-patch+shorthand。
//	@Tags			me
//	@Security		BearerAuth
//	@Accept			application/merge-patch+json,application/json-patch+json,application/merge-patch+shorthand,application/json
//	@Produce		json
//	@Param			body	body		object	true	"patch 主体（具体格式由 Content-Type 决定）"
//	@Success		200		{object}	types.UserInfo
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Router			/api/v1/me [patch]
func (h *MeHandler) Patch(c *echo.Context) error {
	ctx := c.Request().Context()
	user := appctx.MustGetUserFromContext(ctx)

	currentJSON, err := json.Marshal(toUserInfo(user))
	if err != nil {
		return err
	}

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}

	var req types.ReplaceMeRequest
	if err := patch.ApplyInto(currentJSON, body, c.Request().Header.Get(echo.HeaderContentType), &req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	u, err := h.me.Replace(ctx, user.ID, &req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toUserInfo(u))
}

// ChangePassword 修改当前用户密码（自定义子资源动作）。
//
//	@Summary	修改密码
//	@Tags		me
//	@Security	BearerAuth
//	@Accept		json
//	@Produce	json
//	@Param		body	body	types.ChangePasswordRequest	true	"密码信息"
//	@Success	204
//	@Failure	400	{object}	ErrorResponse
//	@Failure	401	{object}	ErrorResponse
//	@Router		/api/v1/me/change-password [post]
func (h *MeHandler) ChangePassword(c *echo.Context) error {
	ctx := c.Request().Context()
	user := appctx.MustGetUserFromContext(ctx)

	var req types.ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	if err := h.me.ChangePassword(ctx, user.ID, &req); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}
