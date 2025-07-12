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

	// 验证token安全性
	if err := s.validateTokenComplete(context.Background(), claims, types.TokenTypeAccess); err != nil {
		return nil, err
	}

	return claims, nil
}

// validateUser 统一验证用户（状态、存在性等）
func (s *AuthService) validateUser(_ context.Context, user *ent.User) error {
	if user == nil {
		return errors.ErrUnauthorized.Errorf("用户不存在")
	}

	if user.Status != userEnt.StatusActive {
		return errors.ErrForbidden.
			With("status", user.Status).
			Errorf("账户已被停用")
	}
	return nil
}

// findUserByEmail 根据邮箱查找用户并验证状态
func (s *AuthService) findUserByEmail(ctx context.Context, email string) (*ent.User, error) {
	user, err := s.orm.User.Query().
		Where(userEnt.Email(email)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrUnauthorized.Errorf("邮箱或密码错误")
		}
		slog.ErrorContext(ctx, "查询用户失败", "error", err, "email", email)
		return nil, errors.ErrDatabase.Wrapf(err, "查询用户失败")
	}

	return user, nil
}

// findUserByID 根据ID查找用户并验证状态
func (s *AuthService) findUserByID(ctx context.Context, userID string) (*ent.User, error) {
	user, err := s.orm.User.Query().
		Where(userEnt.ID(userID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrUnauthorized.Errorf("用户不存在")
		}
		slog.ErrorContext(ctx, "查询用户失败", "error", err, "user_id", userID)
		return nil, errors.ErrDatabase.Wrapf(err, "查询用户失败")
	}

	return user, nil
}

// validateTokenComplete 完整的token验证（类型、过期、安全性）
func (s *AuthService) validateTokenComplete(ctx context.Context, claims *types.JWTClaims, expectedType string) error {
	// 检查token类型
	if claims.TokenType != expectedType {
		slog.WarnContext(ctx, "token类型错误", "expected", expectedType, "actual", claims.TokenType)
		return errors.ErrUnauthorized.Errorf("无效的令牌类型")
	}

	// 检查token是否过期
	if time.Now().After(claims.ExpiresAt.Time) {
		slog.WarnContext(ctx, "token已过期", "user_id", claims.UserID, "type", expectedType)
		return errors.ErrUnauthorized.Errorf("%s令牌已过期", expectedType)
	}

	// 验证token安全性
	if claims.IssuedAt.Time.After(time.Now().Add(time.Minute)) {
		return errors.ErrUnauthorized.Errorf("令牌发行时间异常")
	}

	// 检查token的有效期是否过长
	maxDuration := s.jwtConfig.AccessTokenExpiry
	if claims.TokenType == types.TokenTypeRefresh {
		maxDuration = s.jwtConfig.RefreshTokenExpiry
	}

	if claims.ExpiresAt.Time.After(claims.IssuedAt.Time.Add(maxDuration + time.Hour)) {
		return errors.ErrUnauthorized.Errorf("令牌有效期异常")
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
		return nil, errors.ErrUnauthorized.Errorf("无效的令牌类型")
	}

	dbToken, err := s.orm.Token.Query().
		Where(
			token.Token(tokenString),
			token.IsRevoked(false),
			token.TypeEQ(dbTokenType),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrUnauthorized.Errorf("%s令牌不存在或已撤销", tokenType)
		}
		slog.ErrorContext(ctx, "查询token失败", "error", err, "type", tokenType)
		return nil, errors.ErrDatabase.Wrapf(err, "查询令牌失败")
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
		// 非致命错误，返回原用户对象
		return user, nil
	}
	return updatedUser, nil
}

// saveTokensBatch 批量保存tokens到数据库
func (s *AuthService) saveTokensBatch(ctx context.Context, userID, accessToken, refreshToken string, accessExpiry, refreshExpiry time.Time) error {
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "开启事务失败", "error", err, "user_id", userID)
		return errors.ErrDatabase.Wrapf(err, "开启事务失败")
	}
	defer tx.Rollback()

	// 批量创建tokens
	tokens := []*ent.TokenCreate{
		tx.Token.Create().
			SetID(utils.GenerateULID()).
			SetUserID(userID).
			SetToken(accessToken).
			SetType(token.TypeAccess).
			SetExpiresAt(accessExpiry),
		tx.Token.Create().
			SetID(utils.GenerateULID()).
			SetUserID(userID).
			SetToken(refreshToken).
			SetType(token.TypeRefresh).
			SetExpiresAt(refreshExpiry),
	}

	// 批量保存
	for i, tokenCreate := range tokens {
		if _, err := tokenCreate.Save(ctx); err != nil {
			tokenType := "访问"
			if i == 1 {
				tokenType = "刷新"
			}
			slog.ErrorContext(ctx, "保存"+tokenType+"令牌失败", "error", err, "user_id", userID)
			return errors.ErrDatabase.Wrapf(err, "保存%s令牌失败", tokenType)
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "提交事务失败", "error", err, "user_id", userID)
		return errors.ErrDatabase.Wrapf(err, "提交事务失败")
	}

	return nil
}

// validatePassword 验证密码强度
func (s *AuthService) validatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return errors.ErrBadRequest.Errorf("密码长度至少为%d位", MinPasswordLength)
	}

	hasDigit, hasLetter := false, false
	for _, char := range password {
		if char >= '0' && char <= '9' {
			hasDigit = true
		} else if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			hasLetter = true
		}
		if hasDigit && hasLetter {
			break
		}
	}

	if !hasDigit || !hasLetter {
		return errors.ErrBadRequest.Errorf("密码必须包含字母和数字")
	}

	return nil
}

// checkUserUniqueness 检查用户唯一性
func (s *AuthService) checkUserUniqueness(ctx context.Context, username, email string) error {
	existingUsers, err := s.orm.User.Query().Where(
		userEnt.Or(
			userEnt.Username(username),
			userEnt.Email(email),
		),
	).All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查用户唯一性失败", "error", err, "username", username, "email", email)
		return errors.ErrDatabase.Wrapf(err, "检查用户唯一性失败")
	}

	for _, user := range existingUsers {
		if user.Username == username {
			return errors.ErrConflict.With("username", username).Errorf("用户名已存在")
		}
		if user.Email == email {
			return errors.ErrConflict.With("email", email).Errorf("邮箱已存在")
		}
	}

	return nil
}

// cleanupUserTokens 清理用户过多的tokens
func (s *AuthService) cleanupUserTokens(ctx context.Context, userID string) error {
	// 查找用户的所有有效tokens，按创建时间倒序
	tokens, err := s.orm.Token.Query().
		Where(
			token.UserID(userID),
			token.IsRevoked(false),
		).
		Order(ent.Desc(token.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "查询用户tokens失败", "error", err, "user_id", userID)
		return nil // 非致命错误
	}

	// 如果超过限制，撤销最老的tokens
	if len(tokens) >= MaxTokensPerUser {
		tokensToRevoke := tokens[MaxTokensPerUser-1:]
		for _, t := range tokensToRevoke {
			_, err := s.orm.Token.UpdateOne(t).
				SetIsRevoked(true).
				Save(ctx)
			if err != nil {
				slog.ErrorContext(ctx, "撤销旧token失败", "error", err, "token_id", t.ID)
			}
		}
		slog.InfoContext(ctx, "清理用户多余tokens", "user_id", userID, "revoked_count", len(tokensToRevoke))
	}

	return nil
}

// handleDBError 统一处理数据库错误
func (s *AuthService) handleDBError(ctx context.Context, err error, operation string, details map[string]interface{}) error {
	if err == nil {
		return nil
	}

	// 构建日志字段
	logFields := []interface{}{"operation", operation, "error", err}
	for key, value := range details {
		logFields = append(logFields, key, value)
	}

	if ent.IsNotFound(err) {
		slog.WarnContext(ctx, operation+"：记录不存在", logFields...)
		return errors.ErrNotFound.Errorf("记录不存在")
	}

	if ent.IsConstraintError(err) {
		slog.WarnContext(ctx, operation+"：数据约束冲突", logFields...)
		return errors.ErrConflict.Errorf("数据冲突")
	}

	slog.ErrorContext(ctx, operation+"：数据库操作失败", logFields...)
	return errors.ErrDatabase.Wrapf(err, "%s失败", operation)
}

// Register 用户注册
func (s *AuthService) Register(ctx context.Context, in *types.RegisterInput) (*types.AuthOutput, error) {
	// 验证密码强度
	if err := s.validatePassword(in.Password); err != nil {
		return nil, err
	}

	// 检查用户唯一性
	if err := s.checkUserUniqueness(ctx, in.Username, in.Email); err != nil {
		return nil, err
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
		return nil, s.handleDBError(ctx, err, "创建用户", map[string]interface{}{
			"username": in.Username,
			"email":    in.Email,
		})
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
	user, err := s.findUserByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}

	// 检查用户状态
	if err := s.validateUser(ctx, user); err != nil {
		return nil, err
	}

	// 验证密码
	if err := utils.VerifyPassword(user.PasswordHash, in.Password); err != nil {
		slog.WarnContext(ctx, "用户登录失败：密码错误",
			"user_id", user.ID,
			"email", in.Email,
		)
		return nil, errors.ErrUnauthorized.Errorf("邮箱或密码错误")
	}

	// 清理用户过多的tokens
	go s.cleanupUserTokens(ctx, user.ID)

	// 更新最后登录时间
	user, _ = s.updateLastLoginTime(ctx, user)

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

	// 验证token声明
	if err := s.validateTokenComplete(ctx, claims, types.TokenTypeRefresh); err != nil {
		return nil, err
	}

	// 检查数据库中的token是否存在且未撤销
	dbToken, err := s.findValidToken(ctx, refreshTokenString, types.TokenTypeRefresh)
	if err != nil {
		return nil, err
	}

	// 查找用户并验证状态
	user, err := s.findUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	if err := s.validateUser(ctx, user); err != nil {
		return nil, err
	}

	// 更新token最后使用时间
	s.UpdateTokenUsage(dbToken)

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

	// 保存tokens到数据库 - 使用批量插入优化
	if err := s.saveTokensBatch(ctx, user.ID, accessTokenString, refreshTokenString, accessExpiry, refreshExpiry); err != nil {
		return nil, err
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

// AuthenticateUser 认证用户
// 返回: (user, token, error)
// 成功时返回用户和token信息，error为nil
// 失败时返回nil, nil, error
func (s *AuthService) AuthenticateUser(ctx context.Context, tokenString string) (*ent.User, *ent.Token, error) {
	// 验证JWT token
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		slog.WarnContext(ctx, "认证失败：令牌验证失败", "error", err)
		return nil, nil, errors.ErrUnauthorized.Errorf("无效的访问令牌")
	}

	// 验证token声明
	if err := s.validateTokenComplete(ctx, claims, types.TokenTypeAccess); err != nil {
		return nil, nil, err
	}

	// 检查数据库中的token状态
	dbToken, err := s.findValidToken(ctx, tokenString, types.TokenTypeAccess)
	if err != nil {
		return nil, nil, err
	}

	// 查找用户并验证状态
	user, err := s.findUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, nil, err
	}

	if err := s.validateUser(ctx, user); err != nil {
		return nil, nil, err
	}

	// 认证成功
	return user, dbToken, nil
}

// UpdateTokenUsage 异步更新token使用时间
func (s *AuthService) UpdateTokenUsage(token *ent.Token) {
	if token == nil {
		return
	}

	go func() {
		// 使用带超时的context以避免长时间阻塞
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		now := time.Now()
		_, err := s.orm.Token.UpdateOne(token).
			SetLastUsedAt(now).
			Save(bgCtx)
		if err != nil {
			// 使用结构化日志记录错误
			slog.ErrorContext(bgCtx, "异步更新token使用时间失败",
				"error", err,
				"token_id", token.ID,
				"user_id", token.UserID,
				"token_type", token.Type,
			)
		}
	}()
}

// UpdateTokenUsageSync 同步更新token使用时间
func (s *AuthService) UpdateTokenUsageSync(ctx context.Context, token *ent.Token) error {
	if token == nil {
		return nil
	}

	now := time.Now()
	_, err := s.orm.Token.UpdateOne(token).
		SetLastUsedAt(now).
		Save(ctx)
	if err != nil {
		return s.handleDBError(ctx, err, "更新token使用时间", map[string]interface{}{
			"token_id": token.ID,
			"user_id":  token.UserID,
		})
	}
	return nil
}
