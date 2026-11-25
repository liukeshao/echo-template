package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/token"
	userEnt "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/internal/codes"
	"github.com/liukeshao/echo-template/internal/database"
	"github.com/liukeshao/echo-template/pkg/api"
)

// TokenPair 是认证服务签发的领域令牌对，由 handler 层投影为对外 DTO api.TokenPair。
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
}

// jwtClaims 是 JWT 令牌内部声明（仅含 user_id 与 token_type，不暴露给外部 DTO）。
type jwtClaims struct {
	UserID    string `json:"user_id"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// Auth 认证服务
type Auth struct {
	orm        *ent.Client
	jwtConfig  config.JWT
	bcryptCost int
}

// NewAuth 构造 Auth 并从 DI 容器注入依赖。
func NewAuth(i do.Injector) (*Auth, error) {
	orm := do.MustInvoke[*database.ORM](i)
	cfg := do.MustInvoke[*config.Config](i)
	return &Auth{
		orm:        orm.Client,
		jwtConfig:  cfg.JWT,
		bcryptCost: resolveBcryptCost(cfg.Auth.BcryptCost),
	}, nil
}

// --- 错误构造辅助函数 ---

// authInternal 构造携带 "auth" 标签的 AUTH_INTERNAL_ERROR oops 错误。
// Wrapf message 保持低基数，供 APM 聚合；原始 err 通过 oops 链传递给错误处理器日志。
func authInternal(err error, op string) error {
	return oops.
		Code(codes.AuthInternalError.String()).
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

// authEmailTaken 构造 AUTH_EMAIL_TAKEN oops 错误（HTTP 409）。
func authEmailTaken(email string) error {
	return oops.
		Code(codes.AuthEmailTaken.String()).
		With(codes.CtxKeyParams, map[string]string{"email": email}).
		Public(fmt.Sprintf("the email %q is already registered", email)).
		Errorf("email taken")
}

// authUsernameTaken 构造 AUTH_USERNAME_TAKEN oops 错误（HTTP 409）。
func authUsernameTaken(username string) error {
	return oops.
		Code(codes.AuthUsernameTaken.String()).
		With(codes.CtxKeyParams, map[string]string{"username": username}).
		Public(fmt.Sprintf("the username %q is already taken", username)).
		Errorf("username taken")
}

// --- 私有方法 ---

// parseJWTOfType 解析并验证 JWT 签名、算法、发行者及内置时效（exp/iat/nbf 由库负责），
// 并确认自定义 TokenType claim 与 expectedType 一致。
// 这是所有 token 验证的唯一入口，上层无需再重复时间校验。
func (s *Auth) parseJWTOfType(ctx context.Context, tokenString string, expectedType token.Type) (*jwtClaims, error) {
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
func (s *Auth) generateToken(userID string, tokenType token.Type) (string, time.Time, error) {
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
		ID:        uuid.NewV7().String(),
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Issuer:    s.jwtConfig.Issuer,
		Subject:   userID,
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := t.SignedString([]byte(s.jwtConfig.Secret))
	if err != nil {
		return "", time.Time{}, authInternal(err, "signing token")
	}

	return tokenString, expirationTime, nil
}

// validateUser 验证用户是否处于活跃状态。
func (s *Auth) validateUser(user *ent.User) error {
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
func (s *Auth) findUserByEmail(ctx context.Context, email string) (*ent.User, error) {
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

// saveTokenToDatabase 在事务中持久化 token。
func (s *Auth) saveTokenToDatabase(ctx context.Context, tx *ent.Tx, userID string, tokenString string, tokenType token.Type, expiresAt time.Time) error {
	_, err := tx.Token.Create().
		SetID(uuid.NewV7().String()).
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
func (s *Auth) issueTokenPairTx(ctx context.Context, tx *ent.Tx, userID string) (*TokenPair, error) {
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

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessExpiry.Unix(),
	}, nil
}

// --- 公开方法 ---

// checkUniqueUserConflict 在并发触发唯一约束错误时区分邮箱或用户名冲突。
func (s *Auth) checkUniqueUserConflict(ctx context.Context, email, username string) error {
	if exists, err := s.orm.User.Query().Where(userEnt.Email(email)).Exist(ctx); err == nil && exists {
		return authEmailTaken(email)
	}
	if exists, err := s.orm.User.Query().Where(userEnt.Username(username)).Exist(ctx); err == nil && exists {
		return authUsernameTaken(username)
	}
	return nil
}

// Register 用户注册（在同一事务内创建用户、设置首次登录时间并签发令牌对，避免孤儿用户或后台协程竞争）。
func (s *Auth) Register(ctx context.Context, req *api.RegisterRequest) (_ *TokenPair, err error) {
	emailExists, err := s.orm.User.Query().
		Where(userEnt.Email(req.Email)).
		Exist(ctx)
	if err != nil {
		return nil, authInternal(err, "checking email existence")
	}
	if emailExists {
		return nil, authEmailTaken(req.Email)
	}

	usernameExists, err := s.orm.User.Query().
		Where(userEnt.Username(req.Username)).
		Exist(ctx)
	if err != nil {
		return nil, authInternal(err, "checking username existence")
	}
	if usernameExists {
		return nil, authUsernameTaken(req.Username)
	}

	hashedPassword, err := hashPassword(req.Password, s.bcryptCost)
	if err != nil {
		return nil, authInternal(err, "hashing password")
	}

	tx, err := s.orm.Tx(ctx)
	if err != nil {
		return nil, authInternal(err, "beginning registration transaction")
	}
	rolledBack := false
	defer func() {
		if err != nil && !rolledBack {
			_ = tx.Rollback()
		}
	}()

	userID := uuid.NewV7().String()
	user, err := tx.User.Create().
		SetID(userID).
		SetUsername(req.Username).
		SetEmail(req.Email).
		SetPasswordHash(hashedPassword).
		SetStatus(userEnt.StatusActive).
		SetLastLoginAt(time.Now()).
		Save(ctx)
	if err != nil {
		// 先回滚释放连接（SQLite MaxOpenConns=1），再查询具体冲突列。
		_ = tx.Rollback()
		rolledBack = true
		if ent.IsConstraintError(err) {
			if conflictErr := s.checkUniqueUserConflict(ctx, req.Email, req.Username); conflictErr != nil {
				return nil, conflictErr
			}
		}
		return nil, authInternal(err, "creating user")
	}

	resp, err := s.issueTokenPairTx(ctx, tx, user.ID)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, authInternal(err, "committing registration transaction")
	}

	return resp, nil
}

// Login 用户登录（在同一事务内签发令牌对并更新最后登录时间）。
func (s *Auth) Login(ctx context.Context, req *api.LoginRequest) (_ *TokenPair, err error) {
	user, err := s.findUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}

	// 先校验密码再检查账户状态，防止未认证攻击者枚举已停用账户。
	if err := verifyPassword(user.PasswordHash, req.Password); err != nil {
		slog.WarnContext(ctx, "invalid password attempt", "user_id", user.ID)
		return nil, authInvalidCredentials()
	}

	if err := s.validateUser(user); err != nil {
		return nil, err
	}

	tx, err := s.orm.Tx(ctx)
	if err != nil {
		return nil, authInternal(err, "beginning login transaction")
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	resp, err := s.issueTokenPairTx(ctx, tx, user.ID)
	if err != nil {
		return nil, err
	}

	if err = tx.User.UpdateOneID(user.ID).SetLastLoginAt(time.Now()).Exec(ctx); err != nil {
		return nil, authInternal(err, "updating last login time")
	}

	if err = tx.Commit(); err != nil {
		return nil, authInternal(err, "committing login transaction")
	}

	return resp, nil
}

// RefreshToken 刷新令牌对。
// 在单个事务中原子地撤销旧 refresh token 并签发新令牌对，防止并发重放。
func (s *Auth) RefreshToken(ctx context.Context, req *api.RefreshTokenRequest) (_ *TokenPair, err error) {
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
func (s *Auth) Logout(ctx context.Context, userID string) error {
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
func (s *Auth) AuthenticateUser(ctx context.Context, tokenString string) (*ent.User, error) {
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
