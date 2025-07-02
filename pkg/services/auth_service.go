package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/token"
	userEnt "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// JWTConfig JWT配置结构
type JWTConfig struct {
	Secret             string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
	Issuer             string
}

// NewJWTConfigFromConfig 从应用配置创建JWT配置
func NewJWTConfigFromConfig(cfg config.JWTConfig) JWTConfig {
	return JWTConfig{
		Secret:             cfg.Secret,
		AccessTokenExpiry:  cfg.AccessTokenExpiry,
		RefreshTokenExpiry: cfg.RefreshTokenExpiry,
		Issuer:             cfg.Issuer,
	}
}

// AuthService 认证服务
type AuthService struct {
	orm       *ent.Client
	jwtConfig JWTConfig
}

// NewAuthService 创建认证服务
func NewAuthService(orm *ent.Client, jwtConfig JWTConfig) *AuthService {
	return &AuthService{
		orm:       orm,
		jwtConfig: jwtConfig,
	}
}

// GenerateAccessToken 生成访问令牌
func (s *AuthService) GenerateAccessToken(userID, username, email string) (string, time.Time, error) {
	expirationTime := time.Now().Add(s.jwtConfig.AccessTokenExpiry)

	claims := &types.JWTClaims{
		UserID:    userID,
		Username:  username,
		Email:     email,
		TokenType: types.TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    s.jwtConfig.Issuer,
			Subject:   userID,
			ID:        utils.GenerateULID(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtConfig.Secret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expirationTime, nil
}

// GenerateRefreshToken 生成刷新令牌
func (s *AuthService) GenerateRefreshToken(userID, username, email string) (string, time.Time, error) {
	expirationTime := time.Now().Add(s.jwtConfig.RefreshTokenExpiry)

	claims := &types.JWTClaims{
		UserID:    userID,
		Username:  username,
		Email:     email,
		TokenType: types.TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    s.jwtConfig.Issuer,
			Subject:   userID,
			ID:        utils.GenerateULID(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtConfig.Secret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expirationTime, nil
}

// ValidateToken 验证JWT token
func (s *AuthService) ValidateToken(tokenString string) (*types.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &types.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 检查签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.ErrUnauthorized.Errorf("无效的签名方法")
		}
		return []byte(s.jwtConfig.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*types.JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.ErrUnauthorized.Errorf("无效的token")
	}

	return claims, nil
}

// IsTokenExpired 检查token是否过期
func (s *AuthService) IsTokenExpired(claims *types.JWTClaims) bool {
	return time.Now().After(claims.ExpiresAt.Time)
}

// GetTokenType 获取token类型
func (s *AuthService) GetTokenType(claims *types.JWTClaims) string {
	return claims.TokenType
}

// Register 用户注册
func (s *AuthService) Register(ctx context.Context, in *types.RegisterInput) (*types.AuthOutput, error) {
	// 检查用户名是否已存在
	existingByUsername, err := s.orm.User.Query().
		Where(userEnt.Username(in.Username)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		slog.ErrorContext(ctx, "检查用户名失败", "error", err, "username", in.Username)
		return nil, errors.ErrDatabase.Wrapf(err, "检查用户名失败")
	}
	if existingByUsername != nil {
		return nil, errors.ErrConflict.
			With("username", in.Username).
			Errorf("用户名已存在")
	}

	// 检查邮箱是否已存在
	existingByEmail, err := s.orm.User.Query().
		Where(userEnt.Email(in.Email)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		slog.ErrorContext(ctx, "检查邮箱失败", "error", err, "email", in.Email)
		return nil, errors.ErrDatabase.Wrapf(err, "检查邮箱失败")
	}
	if existingByEmail != nil {
		return nil, errors.ErrConflict.
			With("email", in.Email).
			Errorf("邮箱已存在")
	}

	// 加密密码
	hashedPassword, err := utils.HashPassword(in.Password)
	if err != nil {
		slog.ErrorContext(ctx, "密码加密失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "密码加密失败")
	}

	// 生成用户ID
	userID := utils.GenerateULID()

	// 创建用户
	newUser, err := s.orm.User.Create().
		SetID(userID).
		SetUsername(in.Username).
		SetEmail(in.Email).
		SetPasswordHash(hashedPassword).
		SetStatus(userEnt.StatusActive).
		Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "创建用户失败",
			"error", err,
			"username", in.Username,
			"email", in.Email,
		)
		return nil, errors.ErrDatabase.Wrapf(err, "创建用户失败")
	}

	slog.InfoContext(ctx, "用户注册成功",
		"user_id", newUser.ID,
		"username", newUser.Username,
		"email", newUser.Email,
	)

	// 生成tokens
	return s.generateAuthOutput(ctx, newUser)
}

// Login 用户登录
func (s *AuthService) Login(ctx context.Context, in *types.LoginInput) (*types.AuthOutput, error) {
	// 查找用户
	user, err := s.orm.User.Query().
		Where(userEnt.Email(in.Email)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.WarnContext(ctx, "用户登录失败：邮箱不存在", "email", in.Email)
			return nil, errors.ErrUnauthorized.Errorf("邮箱或密码错误")
		}
		slog.ErrorContext(ctx, "查询用户失败", "error", err, "email", in.Email)
		return nil, errors.ErrDatabase.Wrapf(err, "查询用户失败")
	}

	// 检查用户状态
	if user.Status != userEnt.StatusActive {
		slog.WarnContext(ctx, "用户登录失败：账户状态异常",
			"user_id", user.ID,
			"status", user.Status,
		)
		return nil, errors.ErrForbidden.
			With("status", user.Status).
			Errorf("账户已被停用")
	}

	// 验证密码
	if err := utils.VerifyPassword(user.PasswordHash, in.Password); err != nil {
		slog.WarnContext(ctx, "用户登录失败：密码错误",
			"user_id", user.ID,
			"email", in.Email,
		)
		return nil, errors.ErrUnauthorized.Errorf("邮箱或密码错误")
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
	return s.generateAuthOutput(ctx, user)
}

// RefreshToken 刷新访问令牌
func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenString string) (*types.AuthOutput, error) {
	// 验证refresh token
	claims, err := s.ValidateToken(refreshTokenString)
	if err != nil {
		slog.WarnContext(ctx, "刷新token验证失败", "error", err)
		return nil, errors.ErrUnauthorized.Errorf("无效的刷新令牌")
	}

	// 检查token类型
	if claims.TokenType != types.TokenTypeRefresh {
		slog.WarnContext(ctx, "token类型错误", "token_type", claims.TokenType)
		return nil, errors.ErrUnauthorized.Errorf("无效的令牌类型")
	}

	// 检查token是否过期
	if s.IsTokenExpired(claims) {
		slog.WarnContext(ctx, "刷新token已过期", "user_id", claims.UserID)
		return nil, errors.ErrUnauthorized.Errorf("刷新令牌已过期")
	}

	// 检查数据库中的token是否存在且未撤销
	dbToken, err := s.orm.Token.Query().
		Where(
			token.Token(refreshTokenString),
			token.IsRevoked(false),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.WarnContext(ctx, "刷新token不存在或已撤销", "user_id", claims.UserID)
			return nil, errors.ErrUnauthorized.Errorf("刷新令牌无效")
		}
		slog.ErrorContext(ctx, "查询刷新token失败", "error", err)
		return nil, errors.ErrDatabase.Wrapf(err, "查询令牌失败")
	}

	// 查找用户
	user, err := s.orm.User.Query().
		Where(userEnt.ID(claims.UserID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.WarnContext(ctx, "刷新token对应用户不存在", "user_id", claims.UserID)
			return nil, errors.ErrUnauthorized.Errorf("用户不存在")
		}
		slog.ErrorContext(ctx, "查询用户失败", "error", err, "user_id", claims.UserID)
		return nil, errors.ErrDatabase.Wrapf(err, "查询用户失败")
	}

	// 检查用户状态
	if user.Status != userEnt.StatusActive {
		slog.WarnContext(ctx, "刷新token失败：用户状态异常",
			"user_id", user.ID,
			"status", user.Status,
		)
		return nil, errors.ErrForbidden.Errorf("账户已被停用")
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
	return s.generateAuthOutput(ctx, user)
}

// RevokeToken 撤销令牌
func (s *AuthService) RevokeToken(ctx context.Context, tokenString string) error {
	// 查找token
	dbToken, err := s.orm.Token.Query().
		Where(
			token.Token(tokenString),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.ErrNotFound.Errorf("令牌不存在")
		}
		slog.ErrorContext(ctx, "查询token失败", "error", err)
		return errors.ErrDatabase.Wrapf(err, "查询令牌失败")
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
		return errors.ErrDatabase.Wrapf(err, "撤销令牌失败")
	}

	slog.InfoContext(ctx, "token已撤销",
		"token_id", dbToken.ID,
		"user_id", dbToken.UserID,
	)

	return nil
}

// generateAuthOutput 生成认证输出
func (s *AuthService) generateAuthOutput(ctx context.Context, user *ent.User) (*types.AuthOutput, error) {
	// 生成access token
	accessTokenString, accessExpiry, err := s.GenerateAccessToken(user.ID, user.Username, user.Email)
	if err != nil {
		slog.ErrorContext(ctx, "生成访问令牌失败", "error", err, "user_id", user.ID)
		return nil, errors.ErrInternal.Wrapf(err, "生成访问令牌失败")
	}

	// 生成refresh token
	refreshTokenString, refreshExpiry, err := s.GenerateRefreshToken(user.ID, user.Username, user.Email)
	if err != nil {
		slog.ErrorContext(ctx, "生成刷新令牌失败", "error", err, "user_id", user.ID)
		return nil, errors.ErrInternal.Wrapf(err, "生成刷新令牌失败")
	}

	// 保存tokens到数据库
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "开启事务失败", "error", err)
		return nil, errors.ErrDatabase.Wrapf(err, "开启事务失败")
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
		return nil, errors.ErrDatabase.Wrapf(err, "保存访问令牌失败")
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
		return nil, errors.ErrDatabase.Wrapf(err, "保存刷新令牌失败")
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "提交事务失败", "error", err, "user_id", user.ID)
		return nil, errors.ErrDatabase.Wrapf(err, "提交事务失败")
	}

	return &types.AuthOutput{
		User: &types.UserInfo{
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
