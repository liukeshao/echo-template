package middleware

import (
	"log/slog"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/ent"
	appContext "github.com/liukeshao/echo-template/pkg/context"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// PermissionMiddleware 权限控制中间件
type PermissionMiddleware struct {
	orm         *ent.Client
	roleService *services.RoleService
}

// NewPermissionMiddleware 创建新的权限控制中间件
func NewPermissionMiddleware(orm *ent.Client, roleService *services.RoleService) *PermissionMiddleware {
	return &PermissionMiddleware{
		orm:         orm,
		roleService: roleService,
	}
}

// RequirePermission 需要指定权限的中间件
// permissionCode: 权限代码，如 "user.create", "user.update", "user.delete" 等
func (m *PermissionMiddleware) RequirePermission(permissionCode string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()

			// 从context中获取当前用户（需要先经过认证中间件）
			user, ok := appContext.GetUserFromContext(ctx)
			if !ok || user == nil {
				slog.WarnContext(ctx, "权限检查失败：用户未认证")
				return errors.UnauthorizedError("用户未认证")
			}

			// 检查用户是否拥有指定权限
			hasPermission, err := m.roleService.CheckUserPermission(ctx, user.ID, permissionCode)
			if err != nil {
				slog.ErrorContext(ctx, "权限检查失败",
					"error", err,
					"user_id", user.ID,
					"permission", permissionCode,
				)
				return errors.InternalError("权限检查失败")
			}

			if !hasPermission {
				slog.WarnContext(ctx, "权限不足",
					"user_id", user.ID,
					"username", user.Username,
					"permission", permissionCode,
				)
				return errors.ForbiddenError("权限不足，需要权限：" + permissionCode)
			}

			slog.DebugContext(ctx, "权限检查通过",
				"user_id", user.ID,
				"permission", permissionCode,
			)

			return next(c)
		}
	}
}

// RequireAnyPermission 需要任意一个权限的中间件
// permissionCodes: 权限代码列表，用户拥有其中任意一个即可访问
func (m *PermissionMiddleware) RequireAnyPermission(permissionCodes ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()

			// 从context中获取当前用户
			user, ok := appContext.GetUserFromContext(ctx)
			if !ok || user == nil {
				slog.WarnContext(ctx, "权限检查失败：用户未认证")
				return errors.UnauthorizedError("用户未认证")
			}

			// 检查用户是否拥有任意一个权限
			for _, permissionCode := range permissionCodes {
				hasPermission, err := m.roleService.CheckUserPermission(ctx, user.ID, permissionCode)
				if err != nil {
					slog.ErrorContext(ctx, "权限检查失败",
						"error", err,
						"user_id", user.ID,
						"permission", permissionCode,
					)
					continue
				}

				if hasPermission {
					slog.DebugContext(ctx, "权限检查通过",
						"user_id", user.ID,
						"permission", permissionCode,
					)
					return next(c)
				}
			}

			slog.WarnContext(ctx, "权限不足",
				"user_id", user.ID,
				"username", user.Username,
				"required_permissions", permissionCodes,
			)

			return errors.ForbiddenError("权限不足，需要以下权限之一：" + strings.Join(permissionCodes, ", "))
		}
	}
}

// RequireAllPermissions 需要所有权限的中间件
// permissionCodes: 权限代码列表，用户必须拥有所有权限才能访问
func (m *PermissionMiddleware) RequireAllPermissions(permissionCodes ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()

			// 从context中获取当前用户
			user, ok := appContext.GetUserFromContext(ctx)
			if !ok || user == nil {
				slog.WarnContext(ctx, "权限检查失败：用户未认证")
				return errors.UnauthorizedError("用户未认证")
			}

			// 检查用户是否拥有所有权限
			for _, permissionCode := range permissionCodes {
				hasPermission, err := m.roleService.CheckUserPermission(ctx, user.ID, permissionCode)
				if err != nil {
					slog.ErrorContext(ctx, "权限检查失败",
						"error", err,
						"user_id", user.ID,
						"permission", permissionCode,
					)
					return errors.InternalError("权限检查失败")
				}

				if !hasPermission {
					slog.WarnContext(ctx, "权限不足",
						"user_id", user.ID,
						"username", user.Username,
						"missing_permission", permissionCode,
					)
					return errors.ForbiddenError("权限不足，缺少权限：" + permissionCode)
				}
			}

			slog.DebugContext(ctx, "所有权限检查通过",
				"user_id", user.ID,
				"permissions", permissionCodes,
			)

			return next(c)
		}
	}
}

// CheckMenuPermission 检查菜单权限的中间件
// menuPath: 菜单路径，如 "/system/users", "/system/roles" 等
func (m *PermissionMiddleware) CheckMenuPermission(menuPath string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()

			// 从context中获取当前用户
			user, ok := appContext.GetUserFromContext(ctx)
			if !ok || user == nil {
				slog.WarnContext(ctx, "菜单权限检查失败：用户未认证")
				return errors.UnauthorizedError("用户未认证")
			}

			// 获取用户可访问的菜单
			userMenus, err := m.roleService.GetUserMenus(ctx, &types.GetUserMenusInput{
				UserID:   user.ID,
				TreeMode: false,
				OnlyMenu: false,
			})
			if err != nil {
				slog.ErrorContext(ctx, "获取用户菜单失败",
					"error", err,
					"user_id", user.ID,
				)
				return errors.InternalError("权限检查失败")
			}

			// 检查用户是否有权访问指定菜单
			hasMenuAccess := false
			for _, menu := range userMenus.Menus {
				if menu.Path != nil && (*menu.Path == menuPath || strings.HasPrefix(menuPath, *menu.Path+"/")) {
					hasMenuAccess = true
					break
				}
			}

			if !hasMenuAccess {
				slog.WarnContext(ctx, "菜单权限不足",
					"user_id", user.ID,
					"username", user.Username,
					"menu_path", menuPath,
				)
				return errors.ForbiddenError("无权访问此菜单：" + menuPath)
			}

			slog.DebugContext(ctx, "菜单权限检查通过",
				"user_id", user.ID,
				"menu_path", menuPath,
			)

			return next(c)
		}
	}
}
