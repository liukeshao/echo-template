package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/token"
	userEnt "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/infra"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// jwtClaims 是 JWT 令牌内部声明（仅含 user_id 与 token_type，不暴露给外部 DTO）。
type jwtClaims struct {
	UserID    string `json:"user_id"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// AuthService 认证服务
type AuthService struct {
	orm       *ent.Client
	jwtConfig config.JWTConfig
}

// NewAuthService 构造 AuthService 并从 DI 容器注入依赖。
func NewAuthService(i do.Injector) (*AuthService, error) {
	orm := do.MustInvoke[*infra.ORM](i)
	cfg := do.MustInvoke[*config.Config](i)
	return &AuthService{
		orm:       orm.Client,
		jwtConfig: cfg.JWT,
	}, nil
}

// --- 错误构造辅助函数 ---

// authInternal 构造携带 "auth" 标签的 AUTH_INTERNAL_ERROR oops 错误。
// Wrapf message 保持低基数，供 APM 聚合；原始 err 通过 oops 链传递给错误处理器日志。
func authInternal(err error, op string) error {
	return oops.
		Code(codes.AuthInternal.String()).
		Tags("auth").
		Wrapf(err, "%s", op)
}

// authTokenInvalid 构造 AUTH_TOKEN_INVALID oops 错误。
// public 作为对外 message，不暴露内部细节。
func authTokenInvalid(public string) error {
	return oops.
		Code(codes.AuthTokenInvalid.String()).
		Public(public).
		Errorf("invalid token")
}

// authInvalidCredentials 构造 AUTH_INVALID_CREDENTIALS oops 错误。
// 始终返回相同 public message，防止邮箱枚举攻击。
func authInvalidCredentials() error {
	return oops.
		Code(codes.AuthInvalidCredentials.String()).
		Public("invalid email or password").
		Errorf("invalid credentials")
}

// --- 私有方法 ---

// parseJWTOfType 解析并验证 JWT 签名、算法、发行者及内置时效（exp/iat/nbf 由库负责），
// 并确认自定义 TokenType claim 与 expectedType 一致。
// 这是所有 token 验证的唯一入口，上层无需再重复时间校验。
func (s *AuthService) parseJWTOfType(ctx context.Context, tokenString string, expectedType token.Type) (*jwtClaims, error) {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	}
	if s.jwtConfig.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(s.jwtConfig.Issuer))
	}

	t, err := jwt.ParseWithClaims(tokenString, &jwtClaims{}, func(_ *jwt.Token) (any, error) {
		return []byte(s.jwtConfig.Secret), nil
	}, opts...)
	if err != nil {
		return nil, authTokenInvalid("invalid or expired token")
	}
	claims, ok := t.Claims.(*jwtClaims)
	if !ok || !t.Valid {
		return nil, authTokenInvalid("invalid token claims")
	}
	if token.Type(claims.TokenType) != expectedType {
		slog.WarnContext(ctx, "token type mismatch", "expected", expectedType, "actual", claims.TokenType)
		return nil, authTokenInvalid("invalid token type")
	}
	return claims, nil
}

// generateToken 生成 JWT token（携带唯一 jti，避免同一秒内重复签发触发数据库唯一约束冲突）。
func (s *AuthService) generateToken(userID string, tokenType token.Type) (string, time.Time, error) {
	var expiry time.Duration
	switch tokenType {
	case token.TypeAccess:
		expiry = s.jwtConfig.AccessTokenExpiry
	case token.TypeRefresh:
		expiry = s.jwtConfig.RefreshTokenExpiry
	default:
		return "", time.Time{}, authInternal(oops.Errorf("unknown token type: %s", tokenType), "generating token")
	}

	now := time.Now()
	expirationTime := now.Add(expiry)

	claims := &jwtClaims{
		UserID:    userID,
		TokenType: string(tokenType),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        utils.GenerateULID(),
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.jwtConfig.Issuer,
			Subject:   userID,
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := t.SignedString([]byte(s.jwtConfig.Secret))
	if err != nil {
		return "", time.Time{}, authInternal(err, "signing token")
	}

	return tokenString, expirationTime, nil
}

// validateUser 验证用户是否处于活跃状态。
func (s *AuthService) validateUser(user *ent.User) error {
	if user.Status != userEnt.StatusActive {
		return oops.
			Code(codes.AuthAccountDisabled.String()).
			Public("this account has been disabled").
			Errorf("account disabled: user_id=%s", user.ID)
	}
	return nil
}

// findUserByEmail 根据邮箱查找用户。
// 邮箱不存在时与密码错误返回相同错误，防止用户枚举攻击。
func (s *AuthService) findUserByEmail(ctx context.Context, email string) (*ent.User, error) {
	user, err := s.orm.User.Query().
		Where(userEnt.Email(email)).
		Only(ctx)
	switch {
	case err == nil:
		return user, nil
	case ent.IsNotFound(err):
		slog.WarnContext(ctx, "login attempt with unknown email", "email", email)
		return nil, authInvalidCredentials()
	default:
		return nil, authInternal(err, "querying user by email")
	}
}

// touchLastLogin 异步更新用户最后登录时间（非致命，失败时静默降级）。
// 使用 context.WithoutCancel 配合短超时，防止请求结束后被提前取消或无限期阻塞。
func (s *AuthService) touchLastLogin(ctx context.Context, userID string) {
	go func() {
		detached, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, err := s.orm.User.UpdateOneID(userID).SetLastLoginAt(time.Now()).Save(detached); err != nil {
			slog.WarnContext(detached, "failed to update last login time", "error", err, "user_id", userID)
		}
	}()
}

// saveTokenToDatabase 在事务中持久化 token。
func (s *AuthService) saveTokenToDatabase(ctx context.Context, tx *ent.Tx, userID string, tokenString string, tokenType token.Type, expiresAt time.Time) error {
	_, err := tx.Token.Create().
		SetID(utils.GenerateULID()).
		SetToken(tokenString).
		SetUserID(userID).
		SetType(tokenType).
		SetExpiresAt(expiresAt).
		SetIsRevoked(false).
		Save(ctx)
	if err != nil {
		return authInternal(err, "saving token to database")
	}
	return nil
}

// issueTokenPairTx 在已有事务中签发并持久化 access + refresh token 对。
func (s *AuthService) issueTokenPairTx(ctx context.Context, tx *ent.Tx, userID string) (*types.TokenPair, error) {
	accessToken, accessExpiry, err := s.generateToken(userID, token.TypeAccess)
	if err != nil {
		return nil, err
	}

	refreshToken, refreshExpiry, err := s.generateToken(userID, token.TypeRefresh)
	if err != nil {
		return nil, err
	}

	if err = s.saveTokenToDatabase(ctx, tx, userID, accessToken, token.TypeAccess, accessExpiry); err != nil {
		return nil, err
	}

	if err = s.saveTokenToDatabase(ctx, tx, userID, refreshToken, token.TypeRefresh, refreshExpiry); err != nil {
		return nil, err
	}

	return &types.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessExpiry.Unix(),
	}, nil
}

// generateTokenPair 在独立事务中生成 access + refresh token 对。
// 使用 named return err 配合 defer，保证任意失败路径都触发 Rollback。
func (s *AuthService) generateTokenPair(ctx context.Context, userID string) (_ *types.TokenPair, err error) {
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		return nil, authInternal(err, "beginning transaction")
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	resp, err := s.issueTokenPairTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, authInternal(err, "committing transaction")
	}

	return resp, nil
}

// --- 公开方法 ---

// checkUniqueUserConflict 在并发触发唯一约束错误时区分邮箱或用户名冲突。
func (s *AuthService) checkUniqueUserConflict(ctx context.Context, email, username string) error {
	if exists, err := s.orm.User.Query().Where(userEnt.Email(email)).Exist(ctx); err == nil && exists {
		return oops.
			Code(codes.AuthEmailTaken.String()).
			With(codes.CtxKeyParams, map[string]string{"email": email}).
			Public(fmt.Sprintf("the email %q is already registered", email)).
			Errorf("email taken: %s", email)
	}
	if exists, err := s.orm.User.Query().Where(userEnt.Username(username)).Exist(ctx); err == nil && exists {
		return oops.
			Code(codes.AuthUsernameTaken.String()).
			With(codes.CtxKeyParams, map[string]string{"username": username}).
			Public(fmt.Sprintf("the username %q is already taken", username)).
			Errorf("username taken: %s", username)
	}
	return nil
}

// Register 用户注册。
func (s *AuthService) Register(ctx context.Context, req *types.RegisterRequest) (*types.TokenPair, error) {
	emailExists, err := s.orm.User.Query().
		Where(userEnt.Email(req.Email)).
		Exist(ctx)
	if err != nil {
		return nil, authInternal(err, "checking email existence")
	}
	if emailExists {
		return nil, oops.
			Code(codes.AuthEmailTaken.String()).
			With(codes.CtxKeyParams, map[string]string{"email": req.Email}).
			Public(fmt.Sprintf("the email %q is already registered", req.Email)).
			Errorf("email taken: %s", req.Email)
	}

	usernameExists, err := s.orm.User.Query().
		Where(userEnt.Username(req.Username)).
		Exist(ctx)
	if err != nil {
		return nil, authInternal(err, "checking username existence")
	}
	if usernameExists {
		return nil, oops.
			Code(codes.AuthUsernameTaken.String()).
			With(codes.CtxKeyParams, map[string]string{"username": req.Username}).
			Public(fmt.Sprintf("the username %q is already taken", req.Username)).
			Errorf("username taken: %s", req.Username)
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, authInternal(err, "hashing password")
	}

	userID := utils.GenerateULID()
	user, err := s.orm.User.Create().
		SetID(userID).
		SetUsername(req.Username).
		SetEmail(req.Email).
		SetPasswordHash(hashedPassword).
		SetStatus(userEnt.StatusActive).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			if conflictErr := s.checkUniqueUserConflict(ctx, req.Email, req.Username); conflictErr != nil {
				return nil, conflictErr
			}
		}
		return nil, authInternal(err, "creating user")
	}

	resp, err := s.generateTokenPair(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	s.touchLastLogin(ctx, user.ID)

	return resp, nil
}

// Login 用户登录。
func (s *AuthService) Login(ctx context.Context, req *types.LoginRequest) (*types.TokenPair, error) {
	user, err := s.findUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}

	// 先校验密码再检查账户状态，防止未认证攻击者枚举已停用账户。
	if err := utils.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		slog.WarnContext(ctx, "invalid password attempt", "user_id", user.ID)
		return nil, authInvalidCredentials()
	}

	if err := s.validateUser(user); err != nil {
		return nil, err
	}

	resp, err := s.generateTokenPair(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	s.touchLastLogin(ctx, user.ID)

	return resp, nil
}

// RefreshToken 刷新令牌对。
// 在单个事务中原子地撤销旧 refresh token 并签发新令牌对，防止并发重放。
func (s *AuthService) RefreshToken(ctx context.Context, req *types.RefreshTokenRequest) (_ *types.TokenPair, err error) {
	claims, err := s.parseJWTOfType(ctx, req.RefreshToken, token.TypeRefresh)
	if err != nil {
		return nil, err
	}

	tx, err := s.orm.Tx(ctx)
	if err != nil {
		return nil, authInternal(err, "beginning refresh transaction")
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	revoked, err := tx.Token.Update().
		Where(
			token.Token(req.RefreshToken),
			token.UserID(claims.UserID),
			token.TypeEQ(token.TypeRefresh),
			token.IsRevoked(false),
		).
		SetIsRevoked(true).
		Save(ctx)
	if err != nil {
		return nil, authInternal(err, "revoking old refresh token")
	}
	if revoked == 0 {
		return nil, authTokenInvalid("token not found or has been revoked")
	}

	user, err := tx.User.Query().
		Where(userEnt.ID(claims.UserID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, authTokenInvalid("invalid token: user not found")
		}
		return nil, authInternal(err, "querying user for refresh")
	}

	if err = s.validateUser(user); err != nil {
		return nil, err
	}

	resp, err := s.issueTokenPairTx(ctx, tx, user.ID)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, authInternal(err, "committing refresh transaction")
	}

	return resp, nil
}

// Logout 撤销该用户全部未撤销的令牌（access + refresh），单次原子更新使所有会话同时失效。
func (s *AuthService) Logout(ctx context.Context, userID string) error {
	_, err := s.orm.Token.Update().
		Where(
			token.UserID(userID),
			token.IsRevoked(false),
		).
		SetIsRevoked(true).
		Save(ctx)
	if err != nil {
		return authInternal(err, "revoking user tokens on logout")
	}
	return nil
}

// AuthenticateUser 验证请求中的 access token，供中间件调用。
// 通过单次关联查询同时校验 token 有效性与对应活跃用户，减少数据库往返。
func (s *AuthService) AuthenticateUser(ctx context.Context, tokenString string) (*ent.User, error) {
	claims, err := s.parseJWTOfType(ctx, tokenString, token.TypeAccess)
	if err != nil {
		return nil, err
	}

	user, err := s.orm.Token.Query().
		Where(
			token.Token(tokenString),
			token.UserID(claims.UserID),
			token.IsRevoked(false),
			token.TypeEQ(token.TypeAccess),
		).
		QueryUser().
		Only(ctx)
	switch {
	case err == nil:
	case ent.IsNotFound(err):
		return nil, authTokenInvalid("token not found or has been revoked")
	default:
		return nil, authInternal(err, "authenticating user")
	}

	if err := s.validateUser(user); err != nil {
		return nil, err
	}

	return user, nil
}
