package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/token"
	userEnt "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// AuthService 认证服务
type AuthService struct {
	orm *ent.Client
}

// NewAuthService 创建新的认证服务
func NewAuthService(orm *ent.Client) *AuthService {
	return &AuthService{
		orm: orm,
	}
}

// RegisterRequest 用户注册请求
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// LoginRequest 用户登录请求
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// AuthResponse 认证响应
type AuthResponse struct {
	User         *UserInfo `json:"user"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    int64     `json:"expires_at"`
}

// UserInfo 用户信息
type UserInfo struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Register 用户注册
func (s *AuthService) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	// 验证密码强度
	if !utils.IsPasswordValid(req.Password) {
		return nil, errors.NewValidationError("密码长度至少8位").
			With("field", "password")
	}

	// 检查用户名是否已存在
	existingByUsername, err := s.orm.User.Query().
		Where(userEnt.Username(req.Username), userEnt.DeletedAt(0)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		slog.ErrorContext(ctx, "检查用户名失败", "error", err, "username", req.Username)
		return nil, errors.NewDatabaseError("检查用户名失败").Wrap(err)
	}
	if existingByUsername != nil {
		return nil, errors.ConflictError("用户名已存在").
			With("username", req.Username)
	}

	// 检查邮箱是否已存在
	existingByEmail, err := s.orm.User.Query().
		Where(userEnt.Email(req.Email), userEnt.DeletedAt(0)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		slog.ErrorContext(ctx, "检查邮箱失败", "error", err, "email", req.Email)
		return nil, errors.NewDatabaseError("检查邮箱失败").Wrap(err)
	}
	if existingByEmail != nil {
		return nil, errors.ConflictError("邮箱已存在").
			With("email", req.Email)
	}

	// 加密密码
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		slog.ErrorContext(ctx, "密码加密失败", "error", err)
		return nil, errors.InternalError("密码加密失败").Wrap(err)
	}

	// 生成用户ID
	userID := utils.GenerateULID()

	// 创建用户
	newUser, err := s.orm.User.Create().
		SetID(userID).
		SetUsername(req.Username).
		SetEmail(req.Email).
		SetPasswordHash(hashedPassword).
		SetStatus(userEnt.StatusActive).
		Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "创建用户失败",
			"error", err,
			"username", req.Username,
			"email", req.Email,
		)
		return nil, errors.NewDatabaseError("创建用户失败").Wrap(err)
	}

	slog.InfoContext(ctx, "用户注册成功",
		"user_id", newUser.ID,
		"username", newUser.Username,
		"email", newUser.Email,
	)

	// 生成tokens
	return s.generateAuthResponse(ctx, newUser)
}

// Login 用户登录
func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	// 查找用户
	user, err := s.orm.User.Query().
		Where(userEnt.Email(req.Email), userEnt.DeletedAt(0)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.WarnContext(ctx, "用户登录失败：邮箱不存在", "email", req.Email)
			return nil, errors.UnauthorizedError("邮箱或密码错误")
		}
		slog.ErrorContext(ctx, "查询用户失败", "error", err, "email", req.Email)
		return nil, errors.NewDatabaseError("查询用户失败").Wrap(err)
	}

	// 检查用户状态
	if user.Status != userEnt.StatusActive {
		slog.WarnContext(ctx, "用户登录失败：账户状态异常",
			"user_id", user.ID,
			"status", user.Status,
		)
		return nil, errors.ForbiddenError("账户已被停用").
			With("status", user.Status)
	}

	// 验证密码
	if err := utils.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		slog.WarnContext(ctx, "用户登录失败：密码错误",
			"user_id", user.ID,
			"email", req.Email,
		)
		return nil, errors.UnauthorizedError("邮箱或密码错误")
	}

	// 更新最后登录时间
	now := time.Now()
	user, err = s.orm.User.UpdateOne(user).
		SetLastLoginAt(now).
		Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新最后登录时间失败",
			"error", err,
			"user_id", user.ID,
		)
		// 非致命错误，不影响登录流程
	}

	slog.InfoContext(ctx, "用户登录成功",
		"user_id", user.ID,
		"username", user.Username,
		"email", user.Email,
	)

	// 生成tokens
	return s.generateAuthResponse(ctx, user)
}

// RefreshToken 刷新访问令牌
func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenString string) (*AuthResponse, error) {
	// 验证refresh token
	claims, err := utils.ValidateToken(refreshTokenString)
	if err != nil {
		slog.WarnContext(ctx, "刷新token验证失败", "error", err)
		return nil, errors.UnauthorizedError("无效的刷新令牌")
	}

	// 检查token类型
	if claims.TokenType != "refresh" {
		slog.WarnContext(ctx, "token类型错误", "token_type", claims.TokenType)
		return nil, errors.UnauthorizedError("无效的令牌类型")
	}

	// 检查token是否过期
	if utils.IsTokenExpired(claims) {
		slog.WarnContext(ctx, "刷新token已过期", "user_id", claims.UserID)
		return nil, errors.UnauthorizedError("刷新令牌已过期")
	}

	// 检查数据库中的token是否存在且未撤销
	dbToken, err := s.orm.Token.Query().
		Where(
			token.Token(refreshTokenString),
			token.DeletedAt(0),
			token.IsRevoked(false),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.WarnContext(ctx, "刷新token不存在或已撤销", "user_id", claims.UserID)
			return nil, errors.UnauthorizedError("刷新令牌无效")
		}
		slog.ErrorContext(ctx, "查询刷新token失败", "error", err)
		return nil, errors.NewDatabaseError("查询令牌失败").Wrap(err)
	}

	// 查找用户
	user, err := s.orm.User.Query().
		Where(userEnt.ID(claims.UserID), userEnt.DeletedAt(0)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.WarnContext(ctx, "刷新token对应用户不存在", "user_id", claims.UserID)
			return nil, errors.UnauthorizedError("用户不存在")
		}
		slog.ErrorContext(ctx, "查询用户失败", "error", err, "user_id", claims.UserID)
		return nil, errors.NewDatabaseError("查询用户失败").Wrap(err)
	}

	// 检查用户状态
	if user.Status != userEnt.StatusActive {
		slog.WarnContext(ctx, "刷新token失败：用户状态异常",
			"user_id", user.ID,
			"status", user.Status,
		)
		return nil, errors.ForbiddenError("账户已被停用")
	}

	// 更新token最后使用时间
	now := time.Now()
	_, err = s.orm.Token.UpdateOne(dbToken).
		SetLastUsedAt(now).
		Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新token使用时间失败",
			"error", err,
			"token_id", dbToken.ID,
		)
		// 非致命错误，不影响刷新流程
	}

	slog.InfoContext(ctx, "刷新token成功",
		"user_id", user.ID,
		"username", user.Username,
	)

	// 生成新的tokens
	return s.generateAuthResponse(ctx, user)
}

// RevokeToken 撤销令牌
func (s *AuthService) RevokeToken(ctx context.Context, tokenString string) error {
	// 查找token
	dbToken, err := s.orm.Token.Query().
		Where(
			token.Token(tokenString),
			token.DeletedAt(0),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.NotFoundError("令牌不存在")
		}
		slog.ErrorContext(ctx, "查询token失败", "error", err)
		return errors.NewDatabaseError("查询令牌失败").Wrap(err)
	}

	// 撤销token
	_, err = s.orm.Token.UpdateOne(dbToken).
		SetIsRevoked(true).
		Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "撤销token失败",
			"error", err,
			"token_id", dbToken.ID,
		)
		return errors.NewDatabaseError("撤销令牌失败").Wrap(err)
	}

	slog.InfoContext(ctx, "token已撤销",
		"token_id", dbToken.ID,
		"user_id", dbToken.UserID,
	)

	return nil
}

// generateAuthResponse 生成认证响应
func (s *AuthService) generateAuthResponse(ctx context.Context, user *ent.User) (*AuthResponse, error) {
	// 生成access token
	accessTokenString, accessExpiry, err := utils.GenerateAccessToken(
		user.ID, user.Username, user.Email,
	)
	if err != nil {
		slog.ErrorContext(ctx, "生成访问令牌失败", "error", err, "user_id", user.ID)
		return nil, errors.InternalError("生成访问令牌失败").Wrap(err)
	}

	// 生成refresh token
	refreshTokenString, refreshExpiry, err := utils.GenerateRefreshToken(
		user.ID, user.Username, user.Email,
	)
	if err != nil {
		slog.ErrorContext(ctx, "生成刷新令牌失败", "error", err, "user_id", user.ID)
		return nil, errors.InternalError("生成刷新令牌失败").Wrap(err)
	}

	// 保存tokens到数据库
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "开启事务失败", "error", err)
		return nil, errors.NewDatabaseError("开启事务失败").Wrap(err)
	}
	defer tx.Rollback()

	// 保存access token
	_, err = tx.Token.Create().
		SetID(utils.GenerateULID()).
		SetUserID(user.ID).
		SetToken(accessTokenString).
		SetType(token.TypeAccess).
		SetExpiresAt(accessExpiry).
		Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "保存访问令牌失败", "error", err, "user_id", user.ID)
		return nil, errors.NewDatabaseError("保存访问令牌失败").Wrap(err)
	}

	// 保存refresh token
	_, err = tx.Token.Create().
		SetID(utils.GenerateULID()).
		SetUserID(user.ID).
		SetToken(refreshTokenString).
		SetType(token.TypeRefresh).
		SetExpiresAt(refreshExpiry).
		Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "保存刷新令牌失败", "error", err, "user_id", user.ID)
		return nil, errors.NewDatabaseError("保存刷新令牌失败").Wrap(err)
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "提交事务失败", "error", err, "user_id", user.ID)
		return nil, errors.NewDatabaseError("提交事务失败").Wrap(err)
	}

	return &AuthResponse{
		User: &UserInfo{
			ID:          user.ID,
			Username:    user.Username,
			Email:       user.Email,
			Status:      string(user.Status),
			LastLoginAt: user.LastLoginAt,
			CreatedAt:   user.CreatedAt,
		},
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresAt:    accessExpiry.Unix(),
	}, nil
}
