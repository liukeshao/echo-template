package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/oops"
	"golang.org/x/crypto/bcrypt"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/token"
	userEnt "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/apperrs"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// 安全常量
const (
	MinPasswordLength = 8  // 最小密码长度
	MaxTokensPerUser  = 10 // 每个用户最大token数量
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

// generateToken 生成JWT token（通用方法）
func (s *AuthService) generateToken(ctx context.Context, userID string, tokenType string) (string, time.Time, error) {
	var expiry time.Duration
	switch tokenType {
	case types.TokenTypeAccess:
		expiry = s.jwtConfig.AccessTokenExpiry
	case types.TokenTypeRefresh:
		expiry = s.jwtConfig.RefreshTokenExpiry
	default:
		return "", time.Time{}, apperrs.ErrBadRequest.With("token_type", tokenType).Errorf("无效的令牌类型")
	}

	now := time.Now()
	expirationTime := now.Add(expiry)

	claims := &types.JWTClaims{
		UserID:    userID,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.jwtConfig.Issuer,
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtConfig.Secret))
	if err != nil {
		return "", time.Time{}, apperrs.ErrInternal.With("token_type", tokenType).With("user_id", userID).With("原始错误", err).Errorf("生成%s令牌失败", tokenType)
	}

	return tokenString, expirationTime, nil
}

// ValidateToken 验证JWT token
func (s *AuthService) ValidateToken(tokenString string) (*types.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &types.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 检查签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, apperrs.ErrUnauthorized.Errorf("无效的签名方法")
		}
		return []byte(s.jwtConfig.Secret), nil
	})

	if err != nil {
		return nil, apperrs.ErrUnauthorized.With("原始错误", err).Errorf("JWT 解析失败")
	}

	claims, ok := token.Claims.(*types.JWTClaims)
	if !ok || !token.Valid {
		return nil, apperrs.ErrUnauthorized.Errorf("无效的token")
	}

	// 验证token安全性
	if err := s.validateTokenComplete(context.Background(), claims, types.TokenTypeAccess); err != nil {
		return nil, err
	}

	return claims, nil
}

// validateUser 统一验证用户（状态、存在性等）
func (s *AuthService) validateUser(ctx context.Context, user *ent.User) error {
	if user == nil {
		return apperrs.ErrUnauthorized.Errorf("用户不存在")
	}

	if user.Status != userEnt.StatusActive {
		return apperrs.ErrForbidden.
			With("status", user.Status).
			With("user_id", user.ID).
			Errorf("账户已被停用")
	}
	return nil
}

// findUserByEmail 根据邮箱查找用户并验证状态
func (s *AuthService) findUserByEmail(ctx context.Context, email string) (*ent.User, error) {
	user, err := s.orm.User.Query().
		Where(userEnt.Email(email)).
		Only(ctx)

	switch {
	// 如果实体不满足特定条件，操作将返回 "ent.NotFoundError"
	case ent.IsNotFound(err):
		return nil, apperrs.ErrUnauthorized.With("email", email).Errorf("邮箱不存在")
	// 任何其他错误
	case err != nil:
		slog.ErrorContext(ctx, "查询用户失败", "error", err, "email", email)
		return nil, apperrs.ErrDatabase.With("email", email).With("原始错误", err).Errorf("查询用户失败")
	}

	return user, nil
}

// findUserByID 根据ID查找用户并验证状态
func (s *AuthService) findUserByID(ctx context.Context, userID string) (*ent.User, error) {
	user, err := s.orm.User.Query().
		Where(userEnt.ID(userID)).
		Only(ctx)

	switch {
	// 如果实体不满足特定条件，操作将返回 "ent.NotFoundError"
	case ent.IsNotFound(err):
		return nil, apperrs.ErrUnauthorized.With("user_id", userID).Errorf("用户不存在")
	// 任何其他错误
	case err != nil:
		slog.ErrorContext(ctx, "查询用户失败", "error", err, "user_id", userID)
		return nil, apperrs.ErrDatabase.With("user_id", userID).With("原始错误", err).Errorf("查询用户失败")
	}

	return user, nil
}

// validateTokenComplete 完整的token验证（类型、过期、安全性）
func (s *AuthService) validateTokenComplete(ctx context.Context, claims *types.JWTClaims, expectedType string) error {
	// 检查token类型
	if claims.TokenType != expectedType {
		slog.WarnContext(ctx, "token类型错误", "expected", expectedType, "actual", claims.TokenType)
		return apperrs.ErrUnauthorized.With("expected_type", expectedType).With("actual_type", claims.TokenType).Errorf("无效的令牌类型")
	}

	// 检查token是否过期
	if time.Now().After(claims.ExpiresAt.Time) {
		slog.WarnContext(ctx, "token已过期", "user_id", claims.UserID, "type", expectedType)
		return apperrs.ErrUnauthorized.With("user_id", claims.UserID).With("token_type", expectedType).Errorf("%s令牌已过期", expectedType)
	}

	// 验证token安全性
	if claims.IssuedAt.Time.After(time.Now().Add(time.Minute)) {
		return apperrs.ErrUnauthorized.With("user_id", claims.UserID).With("token_type", expectedType).Errorf("令牌发行时间异常")
	}

	// 检查token的有效期是否过长
	maxDuration := s.jwtConfig.AccessTokenExpiry
	if claims.TokenType == types.TokenTypeRefresh {
		maxDuration = s.jwtConfig.RefreshTokenExpiry
	}

	if claims.ExpiresAt.Time.After(claims.IssuedAt.Time.Add(maxDuration + time.Hour)) {
		return apperrs.ErrUnauthorized.With("user_id", claims.UserID).With("token_type", expectedType).Errorf("令牌有效期异常")
	}

	return nil
}

// findValidToken 查找有效的token记录
func (s *AuthService) findValidToken(ctx context.Context, tokenString string, tokenType string) (*ent.Token, error) {
	var dbTokenType token.Type
	switch tokenType {
	case types.TokenTypeAccess:
		dbTokenType = token.TypeAccess
	case types.TokenTypeRefresh:
		dbTokenType = token.TypeRefresh
	default:
		return nil, apperrs.ErrUnauthorized.Errorf("无效的令牌类型")
	}

	dbToken, err := s.orm.Token.Query().
		Where(
			token.Token(tokenString),
			token.IsRevoked(false),
			token.TypeEQ(dbTokenType),
		).
		Only(ctx)

	switch {
	// 如果实体不满足特定条件，操作将返回 "ent.NotFoundError"
	case ent.IsNotFound(err):
		return nil, apperrs.ErrUnauthorized.With("token_type", tokenType).Errorf("%s令牌不存在或已撤销", tokenType)
	// 任何其他错误
	case err != nil:
		slog.ErrorContext(ctx, "查询token失败", "error", err, "type", tokenType)
		return nil, apperrs.ErrDatabase.With("token_type", tokenType).With("原始错误", err).Errorf("查询令牌失败")
	}
	return dbToken, nil
}

// updateLastLoginTime 更新用户最后登录时间
func (s *AuthService) updateLastLoginTime(ctx context.Context, user *ent.User) (*ent.User, error) {
	now := time.Now()
	updatedUser, err := s.orm.User.UpdateOne(user).
		SetLastLoginAt(now).
		Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新最后登录时间失败",
			"error", err,
			"user_id", user.ID,
		)
		// 非致命错误，返回原用户对象和警告
		slog.WarnContext(ctx, "使用原用户对象，跳过登录时间更新")
		return user, nil
	}

	return updatedUser, nil
}

// saveTokenToDatabase 保存token到数据库
func (s *AuthService) saveTokenToDatabase(ctx context.Context, tx *ent.Tx, userID string, tokenString string, tokenType string, expiresAt time.Time) error {
	var dbTokenType token.Type
	switch tokenType {
	case types.TokenTypeAccess:
		dbTokenType = token.TypeAccess
	case types.TokenTypeRefresh:
		dbTokenType = token.TypeRefresh
	default:
		return oops.Errorf("无效的令牌类型")
	}

	_, err := tx.Token.Create().
		SetToken(tokenString).
		SetUserID(userID).
		SetType(dbTokenType).
		SetExpiresAt(expiresAt).
		SetIsRevoked(false).
		Save(ctx)

	if err != nil {
		return apperrs.ErrDatabase.With("token_type", tokenType).With("user_id", userID).With("原始错误", err).Errorf("保存%s令牌失败", tokenType)
	}

	return nil
}

// generateTokenPair 生成一对token（access + refresh）
func (s *AuthService) generateTokenPair(ctx context.Context, userID string) (*types.AuthOutput, error) {
	// 开始数据库事务
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		return nil, apperrs.ErrDatabase.With("user_id", userID).With("原始错误", err).Errorf("开启事务失败")
	}

	// 生成access token
	accessToken, accessExpiry, err := s.generateToken(ctx, userID, types.TokenTypeAccess)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// 生成refresh token
	refreshToken, refreshExpiry, err := s.generateToken(ctx, userID, types.TokenTypeRefresh)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// 保存access token到数据库
	if err := s.saveTokenToDatabase(ctx, tx, userID, accessToken, types.TokenTypeAccess, accessExpiry); err != nil {
		tx.Rollback()
		return nil, apperrs.ErrDatabase.With("user_id", userID).With("token_type", types.TokenTypeAccess).With("原始错误", err).Errorf("保存访问令牌失败")
	}

	// 保存refresh token到数据库
	if err := s.saveTokenToDatabase(ctx, tx, userID, refreshToken, types.TokenTypeRefresh, refreshExpiry); err != nil {
		tx.Rollback()
		return nil, apperrs.ErrDatabase.With("user_id", userID).With("token_type", types.TokenTypeRefresh).With("原始错误", err).Errorf("保存刷新令牌失败")
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return nil, apperrs.ErrDatabase.With("user_id", userID).With("原始错误", err).Errorf("提交事务失败")
	}

	return &types.AuthOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessExpiry.Unix(),
	}, nil
}

// validatePassword 验证密码强度
func (s *AuthService) validatePassword(ctx context.Context, password string) error {
	if len(password) < MinPasswordLength {
		return apperrs.ErrBadRequest.Errorf("密码长度至少为%d位", MinPasswordLength)
	}
	return nil
}

// Register 用户注册
func (s *AuthService) Register(ctx context.Context, input *types.RegisterInput) (*types.AuthOutput, error) {
	// 密码强度验证
	if err := s.validatePassword(ctx, input.Password); err != nil {
		return nil, err
	}

	// 检查邮箱是否已存在
	exists, err := s.orm.User.Query().
		Where(userEnt.Email(input.Email)).
		Exist(ctx)

	switch {
	case err != nil:
		slog.ErrorContext(ctx, "检查邮箱是否存在失败", "error", err, "email", input.Email)
		return nil, apperrs.ErrDatabase.With("email", input.Email).With("原始错误", err).Errorf("检查邮箱失败")
	}
	if exists {
		return nil, apperrs.ErrConflict.With("email", input.Email).Errorf("邮箱已被注册")
	}

	// 检查用户名是否已存在
	exists, err = s.orm.User.Query().
		Where(userEnt.Username(input.Username)).
		Exist(ctx)

	switch {
	case err != nil:
		slog.ErrorContext(ctx, "检查用户名是否存在失败", "error", err, "username", input.Username)
		return nil, apperrs.ErrDatabase.With("username", input.Username).With("原始错误", err).Errorf("检查用户名失败")
	}
	if exists {
		return nil, apperrs.ErrConflict.With("username", input.Username).Errorf("用户名已被使用")
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "密码加密失败", "error", err)
		return nil, apperrs.ErrInternal.With("username", input.Username).With("email", input.Email).With("原始错误", err).Errorf("密码加密失败")
	}

	// 生成用户ID
	userID := utils.GenerateULID()

	// 创建用户
	user, err := s.orm.User.Create().
		SetID(userID).
		SetUsername(input.Username).
		SetEmail(input.Email).
		SetPasswordHash(string(hashedPassword)).
		SetStatus(userEnt.StatusActive).
		Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "创建用户失败", "error", err, "user_id", userID)
		return nil, apperrs.ErrDatabase.With("user_id", userID).With("username", input.Username).With("email", input.Email).With("原始错误", err).Errorf("创建用户失败")
	}

	// 生成token对
	authOutput, err := s.generateTokenPair(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// 更新最后登录时间
	_, err = s.updateLastLoginTime(ctx, user)
	if err != nil {
		// 非致命错误，记录日志但不影响注册流程
		slog.WarnContext(ctx, "更新最后登录时间失败", "error", err, "user_id", user.ID)
	}

	return authOutput, nil
}

// Login 用户登录
func (s *AuthService) Login(ctx context.Context, input *types.LoginInput) (*types.AuthOutput, error) {
	// 查找用户
	user, err := s.findUserByEmail(ctx, input.Email)
	if err != nil {
		return nil, err
	}

	// 验证用户状态
	if err := s.validateUser(ctx, user); err != nil {
		return nil, err
	}

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
	if err != nil {
		slog.WarnContext(ctx, "密码验证失败", "user_id", user.ID, "email", input.Email)
		return nil, apperrs.ErrUnauthorized.With("user_id", user.ID).With("email", input.Email).Errorf("邮箱或密码错误")
	}

	// 生成token对
	authOutput, err := s.generateTokenPair(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// 更新最后登录时间
	_, err = s.updateLastLoginTime(ctx, user)
	if err != nil {
		// 非致命错误，记录日志但不影响登录流程
		slog.WarnContext(ctx, "更新最后登录时间失败", "error", err, "user_id", user.ID)
	}

	return authOutput, nil
}

// RefreshToken 刷新令牌
func (s *AuthService) RefreshToken(ctx context.Context, input *types.RefreshTokenInput) (*types.AuthOutput, error) {
	// 验证refresh token
	claims, err := jwt.ParseWithClaims(input.RefreshToken, &types.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, apperrs.ErrUnauthorized.Errorf("无效的签名方法")
		}
		return []byte(s.jwtConfig.Secret), nil
	})

	if err != nil {
		return nil, apperrs.ErrUnauthorized.With("原始错误", err).Errorf("refresh token 解析失败")
	}

	jwtClaims, ok := claims.Claims.(*types.JWTClaims)
	if !ok || !claims.Valid {
		return nil, apperrs.ErrUnauthorized.Errorf("无效的refresh token")
	}

	// 验证token完整性
	if err := s.validateTokenComplete(ctx, jwtClaims, types.TokenTypeRefresh); err != nil {
		return nil, err
	}

	// 验证数据库中的token记录
	_, err = s.findValidToken(ctx, input.RefreshToken, types.TokenTypeRefresh)
	if err != nil {
		return nil, err
	}

	// 验证用户状态
	user, err := s.findUserByID(ctx, jwtClaims.UserID)
	if err != nil {
		return nil, err
	}

	if err := s.validateUser(ctx, user); err != nil {
		return nil, err
	}

	// 生成新的token对
	authOutput, err := s.generateTokenPair(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// 撤销旧的refresh token
	if err := s.revokeToken(ctx, input.RefreshToken, types.TokenTypeRefresh); err != nil {
		// 非致命错误，记录日志
		slog.WarnContext(ctx, "撤销旧refresh token失败", "error", err, "user_id", user.ID)
	}

	return authOutput, nil
}

// revokeToken 撤销令牌
func (s *AuthService) revokeToken(ctx context.Context, tokenString string, tokenType string) error {
	var dbTokenType token.Type
	switch tokenType {
	case types.TokenTypeAccess:
		dbTokenType = token.TypeAccess
	case types.TokenTypeRefresh:
		dbTokenType = token.TypeRefresh
	default:
		return apperrs.ErrBadRequest.With("token_type", tokenType).Errorf("无效的令牌类型")
	}

	// 更新token状态为已撤销
	_, err := s.orm.Token.Update().
		Where(
			token.Token(tokenString),
			token.TypeEQ(dbTokenType),
		).
		SetIsRevoked(true).
		Save(ctx)

	if err != nil {
		slog.ErrorContext(ctx, "撤销token失败", "error", err, "type", tokenType)
		return apperrs.ErrDatabase.With("token_type", tokenType).With("原始错误", err).Errorf("撤销令牌失败")
	}

	return nil
}

// Logout 用户登出
func (s *AuthService) Logout(ctx context.Context, accessToken string) error {
	// 验证access token
	claims, err := s.ValidateToken(accessToken)
	if err != nil {
		return err
	}

	// 撤销access token
	if err := s.revokeToken(ctx, accessToken, types.TokenTypeAccess); err != nil {
		return err
	}

	// 撤销用户所有的refresh token
	userID := claims.UserID
	_, err = s.orm.Token.Update().
		Where(
			token.UserID(userID),
			token.TypeEQ(token.TypeRefresh),
			token.IsRevoked(false),
		).
		SetIsRevoked(true).
		Save(ctx)

	if err != nil {
		slog.ErrorContext(ctx, "撤销refresh token失败", "error", err, "user_id", userID)
		return apperrs.ErrDatabase.With("user_id", userID).With("原始错误", err).Errorf("撤销刷新令牌失败")
	}

	return nil
}

// AuthenticateUser 认证用户 - 用于中间件
func (s *AuthService) AuthenticateUser(ctx context.Context, tokenString string) (*ent.User, *ent.Token, error) {
	// 验证 JWT token
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return nil, nil, err
	}

	// 验证数据库中的token记录
	dbToken, err := s.findValidToken(ctx, tokenString, types.TokenTypeAccess)
	if err != nil {
		return nil, nil, err
	}

	// 获取用户信息
	user, err := s.findUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, nil, err
	}

	// 验证用户状态
	if err := s.validateUser(ctx, user); err != nil {
		return nil, nil, err
	}

	return user, dbToken, nil
}

// UpdateTokenUsage 更新token使用时间 - 用于中间件
func (s *AuthService) UpdateTokenUsage(token *ent.Token) {
	// 这是一个非阻塞的异步操作，用于更新token的最后使用时间
	// 在实际项目中，这里可以实现token使用统计等功能
	// 目前为了简化，我们只记录日志
	slog.Debug("Token usage recorded", "token_id", token.ID, "user_id", token.UserID)
}
