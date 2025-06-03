package handlers

import (
	"log/slog"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	orm         *ent.Client
	authService *services.AuthService
}

// 自动注册
func init() {
	Register(new(AuthHandler))
}

// Init 依赖注入
func (h *AuthHandler) Init(c *services.Container) error {
	h.orm = c.ORM
	h.authService = c.Auth
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
	authMiddleware := middleware.NewAuthMiddleware(h.orm)
	protected := g.Group("/api/v1/auth")
	protected.Use(authMiddleware.RequireAuth)
	protected.POST("/logout", h.Logout)
}

// Register 用户注册
func (h *AuthHandler) Register(c echo.Context) error {
	ctx := c.Request().Context()

	var req services.RegisterRequest
	if err := c.Bind(&req); err != nil {
		return errors.BadRequestError("请求参数格式错误").
			With("error", err.Error())
	}

	// 基本验证
	var errorDetails []ErrorDetail
	if req.Username == "" {
		errorDetails = append(errorDetails, ErrorDetail{
			Field:   "username",
			Message: "用户名不能为空",
			Code:    "REQUIRED",
		})
	} else if len(req.Username) < 3 || len(req.Username) > 50 {
		errorDetails = append(errorDetails, ErrorDetail{
			Field:   "username",
			Message: "用户名长度必须在3-50个字符之间",
			Code:    "INVALID_LENGTH",
		})
	}

	if req.Email == "" {
		errorDetails = append(errorDetails, ErrorDetail{
			Field:   "email",
			Message: "邮箱不能为空",
			Code:    "REQUIRED",
		})
	}

	if req.Password == "" {
		errorDetails = append(errorDetails, ErrorDetail{
			Field:   "password",
			Message: "密码不能为空",
			Code:    "REQUIRED",
		})
	} else if len(req.Password) < 8 {
		errorDetails = append(errorDetails, ErrorDetail{
			Field:   "password",
			Message: "密码长度至少8位",
			Code:    "PASSWORD_TOO_SHORT",
		})
	}

	if len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	slog.InfoContext(ctx, "开始用户注册",
		"username", req.Username,
		"email", req.Email,
	)

	// 调用服务层
	response, err := h.authService.Register(ctx, &req)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "用户注册成功",
		"user_id", response.User.ID,
		"username", response.User.Username,
	)

	return Success(response).JSON(c)
}

// Login 用户登录
func (h *AuthHandler) Login(c echo.Context) error {
	ctx := c.Request().Context()

	var req services.LoginRequest
	if err := c.Bind(&req); err != nil {
		return errors.BadRequestError("请求参数格式错误").
			With("error", err.Error())
	}

	// 基本验证
	var errorDetails []ErrorDetail
	if req.Email == "" {
		errorDetails = append(errorDetails, ErrorDetail{
			Field:   "email",
			Message: "邮箱不能为空",
			Code:    "REQUIRED",
		})
	}

	if req.Password == "" {
		errorDetails = append(errorDetails, ErrorDetail{
			Field:   "password",
			Message: "密码不能为空",
			Code:    "REQUIRED",
		})
	}

	if len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	slog.InfoContext(ctx, "用户登录请求", "email", req.Email)

	// 调用服务层
	response, err := h.authService.Login(ctx, &req)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "用户登录成功",
		"user_id", response.User.ID,
		"username", response.User.Username,
	)

	return Success(response).JSON(c)
}

// RefreshToken 刷新访问令牌
func (h *AuthHandler) RefreshToken(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.Bind(&req); err != nil {
		return errors.BadRequestError("请求参数格式错误").
			With("error", err.Error())
	}

	if req.RefreshToken == "" {
		return ValidationError("验证失败", []ErrorDetail{
			{
				Field:   "refresh_token",
				Message: "刷新令牌不能为空",
				Code:    "REQUIRED",
			},
		}).JSON(c)
	}

	slog.InfoContext(ctx, "刷新令牌请求")

	// 调用服务层
	response, err := h.authService.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "令牌刷新成功", "user_id", response.User.ID)

	return Success(response).JSON(c)
}

// Logout 用户登出（撤销令牌）
func (h *AuthHandler) Logout(c echo.Context) error {
	ctx := c.Request().Context()

	// 从认证中间件获取当前用户
	user, ok := middleware.GetUserFromEcho(c)
	if !ok {
		return errors.UnauthorizedError("用户未登录")
	}

	// 从Authorization header获取token
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return errors.UnauthorizedError("缺少Authorization头")
	}

	// 检查Bearer格式
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return errors.UnauthorizedError("无效的Authorization格式")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return errors.UnauthorizedError("令牌不能为空")
	}

	slog.InfoContext(ctx, "用户登出请求", "user_id", user.ID)

	// 撤销token
	err := h.authService.RevokeToken(ctx, token)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "用户登出成功", "user_id", user.ID)

	return Success(map[string]string{
		"message": "登出成功",
	}).JSON(c)
}
