package handlers

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/context"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	orm  *ent.Client
	auth *services.AuthService
}

// 自动注册
func init() {
	Register(new(AuthHandler))
}

// Init 依赖注入
func (h *AuthHandler) Init(c *services.Container) error {
	h.orm = c.ORM
	h.auth = c.Auth
	return nil
}

// Routes 路由定义
func (h *AuthHandler) Routes(g *echo.Group) {
	auth := g.Group("/api/v1/auth")

	// 公开路由（无需认证）
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.POST("/refresh", h.RefreshToken)

	// 需要认证的路由
	authMiddleware := middleware.NewAuthMiddleware(h.orm, h.auth)
	protected := g.Group("/api/v1/auth")
	protected.Use(authMiddleware.RequireAuth)
	protected.POST("/logout", h.Logout)
}

// Register 用户注册
func (h *AuthHandler) Register(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.RegisterInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	if errs := in.Validate(); len(errs) > 0 {
		return ValidationError(c, errs)
	}

	// 调用服务层
	out, err := h.auth.Register(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// Login 用户登录
func (h *AuthHandler) Login(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.LoginInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	if errs := in.Validate(); len(errs) > 0 {
		return ValidationError(c, errs)
	}

	// 调用服务层
	out, err := h.auth.Login(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// RefreshToken 刷新访问令牌
func (h *AuthHandler) RefreshToken(c echo.Context) error {
	ctx := c.Request().Context()

	var req types.RefreshTokenInput
	if err := c.Bind(&req); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errs := req.Validate(); len(errs) > 0 {
		return ValidationError(c, errs)
	}

	// 调用服务层
	out, err := h.auth.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// Logout 用户登出（撤销令牌）
func (h *AuthHandler) Logout(c echo.Context) error {
	ctx := c.Request().Context()

	// 从认证中间件获取当前用户
	_, ok := context.GetUserFromEcho(c)
	if !ok {
		return errors.ErrUnauthorized.Errorf("用户未登录")
	}

	// 从Authorization header获取token
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return errors.ErrUnauthorized.Errorf("缺少Authorization头")
	}

	// 检查Bearer格式
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return errors.ErrUnauthorized.Errorf("无效的Authorization格式")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return errors.ErrUnauthorized.Errorf("令牌不能为空")
	}

	// 撤销token
	err := h.auth.RevokeToken(ctx, token)
	if err != nil {
		return err
	}

	return Success(c, nil)
}
