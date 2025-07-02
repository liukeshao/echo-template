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
	appContext "github.com/liukeshao/echo-template/pkg/context"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/types"
)

// AuthService 认证服务接口
type AuthService interface {
	ValidateToken(tokenString string) (*types.JWTClaims, error)
	IsTokenExpired(claims *types.JWTClaims) bool
	GetTokenType(claims *types.JWTClaims) string
}

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	orm         *ent.Client
	authService AuthService
}

// AuthResult 认证结果
type AuthResult struct {
	User    *ent.User  // 认证成功的用户
	Token   *ent.Token // 数据库中的token记录
	Error   error      // 认证错误
	IsValid bool       // 是否认证成功
}

// NewAuthMiddleware 创建新的认证中间件
func NewAuthMiddleware(orm *ent.Client, authService AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		orm:         orm,
		authService: authService,
	}
}

// extractAndValidateToken 提取并验证token的公共逻辑
func (m *AuthMiddleware) extractAndValidateToken(c echo.Context, strictMode bool) *AuthResult {
	ctx := c.Request().Context()
	result := &AuthResult{IsValid: false}

	// 从Authorization header获取token
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		if strictMode {
			slog.WarnContext(ctx, "认证失败：缺少Authorization头")
			result.Error = errors.ErrUnauthorized.Errorf("缺少Authorization头")
		}
		return result
	}

	// 检查Bearer格式
	if !strings.HasPrefix(authHeader, "Bearer ") {
		if strictMode {
			slog.WarnContext(ctx, "认证失败：无效的Authorization格式")
			result.Error = errors.ErrUnauthorized.Errorf("无效的Authorization格式")
		}
		return result
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == "" {
		if strictMode {
			slog.WarnContext(ctx, "认证失败：令牌为空")
			result.Error = errors.ErrUnauthorized.Errorf("令牌不能为空")
		}
		return result
	}

	// 验证JWT token
	claims, err := m.authService.ValidateToken(tokenString)
	if err != nil {
		if strictMode {
			slog.WarnContext(ctx, "认证失败：令牌验证失败", "error", err)
			result.Error = errors.ErrUnauthorized.Errorf("无效的访问令牌")
		}
		return result
	}

	// 检查token类型
	if m.authService.GetTokenType(claims) != types.TokenTypeAccess {
		if strictMode {
			slog.WarnContext(ctx, "认证失败：令牌类型错误", "token_type", claims.TokenType)
			result.Error = errors.ErrUnauthorized.Errorf("无效的令牌类型")
		}
		return result
	}

	// 检查token是否过期
	if m.authService.IsTokenExpired(claims) {
		if strictMode {
			slog.WarnContext(ctx, "认证失败：令牌已过期", "user_id", claims.UserID)
			result.Error = errors.ErrUnauthorized.Errorf("访问令牌已过期")
		}
		return result
	}

	// 严格模式下检查数据库中的token状态
	if strictMode {
		dbToken, err := m.orm.Token.Query().
			Where(
				token.Token(tokenString),
				token.IsRevoked(false),
				token.TypeEQ(token.TypeAccess),
			).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				slog.WarnContext(ctx, "认证失败：令牌不存在或已撤销", "user_id", claims.UserID)
				result.Error = errors.ErrUnauthorized.Errorf("访问令牌无效")
			} else {
				slog.ErrorContext(ctx, "认证失败：查询令牌失败", "error", err)
				result.Error = errors.ErrInternal.Errorf("系统错误")
			}
			return result
		}
		result.Token = dbToken
	}

	// 查找用户
	user, err := m.orm.User.Query().
		Where(userEnt.ID(claims.UserID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			if strictMode {
				slog.WarnContext(ctx, "认证失败：用户不存在", "user_id", claims.UserID)
				result.Error = errors.ErrUnauthorized.Errorf("用户不存在")
			}
		} else {
			if strictMode {
				slog.ErrorContext(ctx, "认证失败：查询用户失败", "error", err, "user_id", claims.UserID)
				result.Error = errors.ErrInternal.Errorf("系统错误")
			}
		}
		return result
	}

	// 检查用户状态
	if user.Status != userEnt.StatusActive {
		if strictMode {
			slog.WarnContext(ctx, "认证失败：用户状态异常",
				"user_id", user.ID,
				"status", user.Status,
			)
			result.Error = errors.ErrForbidden.Errorf("账户已被停用")
		}
		return result
	}

	// 认证成功
	result.User = user
	result.IsValid = true
	return result
}

// setUserContext 将用户信息存储到context中
func (m *AuthMiddleware) setUserContext(c echo.Context, user *ent.User) {
	ctx := c.Request().Context()
	userCtx := appContext.WithUser(ctx, user)
	c.SetRequest(c.Request().WithContext(userCtx))
}

// updateTokenUsage 异步更新token使用时间
func (m *AuthMiddleware) updateTokenUsage(token *ent.Token) {
	if token == nil {
		return
	}

	go func() {
		// 使用新的context以避免父context被取消时影响更新
		bgCtx := context.Background()
		now := time.Now()
		_, err := m.orm.Token.UpdateOne(token).
			SetLastUsedAt(now).
			Save(bgCtx)
		if err != nil {
			slog.ErrorContext(bgCtx, "更新token使用时间失败",
				"error", err,
				"token_id", token.ID,
			)
		}
	}()
}

// RequireAuth 要求用户认证的中间件
func (m *AuthMiddleware) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		// 使用严格模式进行认证验证
		result := m.extractAndValidateToken(c, true)

		// 认证失败，返回错误
		if !result.IsValid {
			return result.Error
		}

		// 更新token使用时间（仅在严格模式下有token记录时）
		m.updateTokenUsage(result.Token)

		// 将用户信息存储到context中
		m.setUserContext(c, result.User)

		slog.DebugContext(ctx, "用户认证成功",
			"user_id", result.User.ID,
			"username", result.User.Username,
		)

		return next(c)
	}
}

// OptionalAuth 可选认证中间件（不强制要求认证）
func (m *AuthMiddleware) OptionalAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		// 使用非严格模式进行认证验证
		result := m.extractAndValidateToken(c, false)

		// 认证成功时，将用户信息存储到context中
		if result.IsValid {
			m.setUserContext(c, result.User)

			slog.DebugContext(ctx, "可选认证成功",
				"user_id", result.User.ID,
				"username", result.User.Username,
			)
		}

		// 无论认证是否成功，都继续处理请求
		return next(c)
	}
}
