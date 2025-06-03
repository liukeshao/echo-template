package middleware

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/token"
	userEnt "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// UserContext 用户上下文键
type contextKey string

const (
	UserContextKey contextKey = "user"
)

// AuthMiddleware 认证中间件配置
type AuthMiddleware struct {
	orm *ent.Client
}

// NewAuthMiddleware 创建新的认证中间件
func NewAuthMiddleware(orm *ent.Client) *AuthMiddleware {
	return &AuthMiddleware{
		orm: orm,
	}
}

// RequireAuth 要求用户认证的中间件
func (m *AuthMiddleware) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		// 从Authorization header获取token
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			slog.WarnContext(ctx, "认证失败：缺少Authorization头")
			return errors.UnauthorizedError("缺少Authorization头")
		}

		// 检查Bearer格式
		if !strings.HasPrefix(authHeader, "Bearer ") {
			slog.WarnContext(ctx, "认证失败：无效的Authorization格式")
			return errors.UnauthorizedError("无效的Authorization格式")
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			slog.WarnContext(ctx, "认证失败：令牌为空")
			return errors.UnauthorizedError("令牌不能为空")
		}

		// 验证JWT token
		claims, err := utils.ValidateToken(tokenString)
		if err != nil {
			slog.WarnContext(ctx, "认证失败：令牌验证失败", "error", err)
			return errors.UnauthorizedError("无效的访问令牌")
		}

		// 检查token类型
		if claims.TokenType != "access" {
			slog.WarnContext(ctx, "认证失败：令牌类型错误", "token_type", claims.TokenType)
			return errors.UnauthorizedError("无效的令牌类型")
		}

		// 检查token是否过期
		if utils.IsTokenExpired(claims) {
			slog.WarnContext(ctx, "认证失败：令牌已过期", "user_id", claims.UserID)
			return errors.UnauthorizedError("访问令牌已过期")
		}

		// 检查数据库中的token是否存在且未撤销
		dbToken, err := m.orm.Token.Query().
			Where(
				token.Token(tokenString),
				token.DeletedAt(0),
				token.IsRevoked(false),
				token.TypeEQ(token.TypeAccess),
			).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				slog.WarnContext(ctx, "认证失败：令牌不存在或已撤销", "user_id", claims.UserID)
				return errors.UnauthorizedError("访问令牌无效")
			}
			slog.ErrorContext(ctx, "认证失败：查询令牌失败", "error", err)
			return errors.InternalError("系统错误")
		}

		// 查找用户
		user, err := m.orm.User.Query().
			Where(userEnt.ID(claims.UserID), userEnt.DeletedAt(0)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				slog.WarnContext(ctx, "认证失败：用户不存在", "user_id", claims.UserID)
				return errors.UnauthorizedError("用户不存在")
			}
			slog.ErrorContext(ctx, "认证失败：查询用户失败", "error", err, "user_id", claims.UserID)
			return errors.InternalError("系统错误")
		}

		// 检查用户状态
		if user.Status != userEnt.StatusActive {
			slog.WarnContext(ctx, "认证失败：用户状态异常",
				"user_id", user.ID,
				"status", user.Status,
			)
			return errors.ForbiddenError("账户已被停用")
		}

		// 更新token最后使用时间（异步处理，不影响请求性能）
		go func() {
			// 使用新的context以避免父context被取消时影响更新
			bgCtx := context.Background()
			now := time.Now()
			_, err := m.orm.Token.UpdateOne(dbToken).
				SetLastUsedAt(now).
				Save(bgCtx)
			if err != nil {
				slog.ErrorContext(bgCtx, "更新token使用时间失败",
					"error", err,
					"token_id", dbToken.ID,
				)
			}
		}()

		// 将用户信息存储到context中
		userCtx := context.WithValue(ctx, UserContextKey, user)
		c.SetRequest(c.Request().WithContext(userCtx))

		slog.DebugContext(ctx, "用户认证成功",
			"user_id", user.ID,
			"username", user.Username,
		)

		return next(c)
	}
}

// OptionalAuth 可选认证中间件（不强制要求认证）
func (m *AuthMiddleware) OptionalAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		// 从Authorization header获取token
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			// 没有认证信息，继续处理请求
			return next(c)
		}

		// 检查Bearer格式
		if !strings.HasPrefix(authHeader, "Bearer ") {
			// 格式不正确，继续处理请求（不返回错误）
			return next(c)
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			// 令牌为空，继续处理请求
			return next(c)
		}

		// 验证JWT token
		claims, err := utils.ValidateToken(tokenString)
		if err != nil {
			// 令牌无效，继续处理请求（不返回错误）
			return next(c)
		}

		// 检查token类型和过期时间
		if claims.TokenType != "access" || utils.IsTokenExpired(claims) {
			// 令牌类型错误或已过期，继续处理请求
			return next(c)
		}

		// 查找用户（简化版本，不检查token在数据库中的状态）
		user, err := m.orm.User.Query().
			Where(userEnt.ID(claims.UserID), userEnt.DeletedAt(0)).
			Only(ctx)
		if err != nil {
			// 用户不存在，继续处理请求
			return next(c)
		}

		// 检查用户状态
		if user.Status != userEnt.StatusActive {
			// 用户状态异常，继续处理请求
			return next(c)
		}

		// 将用户信息存储到context中
		userCtx := context.WithValue(ctx, UserContextKey, user)
		c.SetRequest(c.Request().WithContext(userCtx))

		slog.DebugContext(ctx, "可选认证成功",
			"user_id", user.ID,
			"username", user.Username,
		)

		return next(c)
	}
}

// GetUserFromContext 从context中获取当前用户
func GetUserFromContext(ctx context.Context) (*ent.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*ent.User)
	return user, ok
}

// GetUserFromEcho 从Echo context中获取当前用户
func GetUserFromEcho(c echo.Context) (*ent.User, bool) {
	return GetUserFromContext(c.Request().Context())
}

// MustGetUser 从context中获取用户，如果不存在则panic（用于必须有用户的地方）
func MustGetUser(ctx context.Context) *ent.User {
	user, ok := GetUserFromContext(ctx)
	if !ok {
		panic("user not found in context")
	}
	return user
}
