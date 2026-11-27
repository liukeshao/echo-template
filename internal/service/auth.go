package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"time"
	"uuid"

	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/refreshtoken"
	entuser "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/internal/auth"
	"github.com/liukeshao/echo-template/internal/database"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/httperr"
)

var errAuth = oops.In("auth")

// TokenPair 是认证服务签发的领域令牌对，由 handler 层投影为对外 DTO api.TokenPair。
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
}

// Auth 认证与凭证领域服务（负责注册、登录、令牌轮换、登出与改密，唯一持有 bcrypt 与 refresh_tokens 表操作）。
type Auth struct {
	orm                *ent.Client
	jwt                *auth.JWT
	refreshTokenExpiry time.Duration
	bcryptCost         int
	dummyPasswordHash  string
}

// NewAuth 构造 Auth 并从 DI 容器注入依赖。
func NewAuth(i do.Injector) (*Auth, error) {
	orm := do.MustInvoke[*database.ORM](i)
	cfg := do.MustInvoke[*config.Config](i)
	jwtMgr := do.MustInvoke[*auth.JWT](i)

	cost := resolveBcryptCost(cfg.Auth.BcryptCost)
	dummyHash, err := hashPassword("dummy-password-for-constant-time-login", cost)
	if err != nil {
		return nil, authInternal(err, "initializing dummy password hash")
	}

	return &Auth{
		orm:                orm.Client,
		jwt:                jwtMgr,
		refreshTokenExpiry: cfg.JWT.RefreshTokenExpiry,
		bcryptCost:         cost,
		dummyPasswordHash:  dummyHash,
	}, nil
}

// --- 错误构造辅助函数（有限 httperr.Status 哨兵 + 原生 oops + AIP-193 Metadata） ---

func authInternal(err error, op string) error {
	return errAuth.
		With("op", op).
		Join(httperr.Internal, err)
}

func refreshTokenError(reason string, cause error) error {
	b := errAuth.With(httperr.MetaKey, httperr.PreconditionInfo{
		Type:    reason,
		Subject: "refresh_token",
	})
	if cause != nil {
		return b.Join(httperr.Unauthorized, cause)
	}
	return b.Wrap(httperr.Unauthorized)
}

// authInvalidCredentials 故意不区分"邮箱不存在"与"密码错误"，防止账户枚举攻击。
func authInvalidCredentials() error {
	return errAuth.
		With(httperr.MetaKey, httperr.PreconditionInfo{
			Type: httperr.PreconditionInvalidCredentials,
		}).
		Wrap(httperr.Unauthorized)
}

// --- 私有方法 ---

func generateOpaqueRefreshToken() (raw, hash string, err error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf[:])
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Auth) validateUser(user *ent.User) error {
	if user.Status != entuser.StatusActive {
		return errAuth.
			User(user.ID).
			With(httperr.MetaKey, httperr.PreconditionInfo{
				Type:    httperr.PreconditionAccountDisabled,
				Subject: "account",
				Detail:  string(user.Status),
			}).
			Wrap(httperr.Forbidden)
	}
	return nil
}

func (s *Auth) issueTokenPairTx(ctx context.Context, tx *ent.Tx, userID, familyID string) (*TokenPair, error) {
	if familyID == "" {
		familyID = uuid.NewV7().String()
	}

	// Access Token 为短生命周期无状态 JWT，不落库。
	accessToken, accessExpiry, err := s.jwt.SignAccessToken(userID)
	if err != nil {
		return nil, err
	}

	// Refresh Token 为高熵随机不透明令牌，库中仅存 SHA-256 哈希。
	rawRefresh, refreshHash, err := generateOpaqueRefreshToken()
	if err != nil {
		return nil, authInternal(err, "generating refresh token")
	}

	_, err = tx.RefreshToken.Create().
		SetID(uuid.NewV7().String()).
		SetUserID(userID).
		SetFamilyID(familyID).
		SetTokenHash(refreshHash).
		SetExpiresAt(time.Now().Add(s.refreshTokenExpiry)).
		SetIsRevoked(false).
		Save(ctx)
	if err != nil {
		return nil, authInternal(err, "saving refresh token")
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresAt:    accessExpiry.Unix(),
	}, nil
}

// --- 公开方法 ---

// Register 用户注册（在同一事务内创建用户、设置首次登录时间并签发令牌对）。
func (s *Auth) Register(ctx context.Context, req *api.RegisterRequest) (*TokenPair, error) {
	if err := checkUserUnique(ctx, s.orm, errAuth, "", req.Email, req.Username, authInternal); err != nil {
		return nil, err
	}

	hashedPassword, err := hashPassword(req.Password, s.bcryptCost)
	if err != nil {
		return nil, authInternal(err, "hashing password")
	}

	var (
		resp       *TokenPair
		constraint bool
	)
	err = withTx(ctx, s.orm, authInternal, func(tx *ent.Tx) error {
		user, err := tx.User.Create().
			SetID(uuid.NewV7().String()).
			SetUsername(req.Username).
			SetEmail(req.Email).
			SetPasswordHash(hashedPassword).
			SetStatus(entuser.StatusActive).
			SetLastLoginAt(time.Now()).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				constraint = true
			}
			return authInternal(err, "creating user")
		}
		resp, err = s.issueTokenPairTx(ctx, tx, user.ID, "")
		return err
	})
	if err != nil {
		if constraint {
			if conflictErr := checkUserUnique(ctx, s.orm, errAuth, "", req.Email, req.Username, nil); conflictErr != nil {
				return nil, conflictErr
			}
		}
		return nil, err
	}

	return resp, nil
}

// Login 用户登录（在同一事务内签发令牌对并更新最后登录时间）。
func (s *Auth) Login(ctx context.Context, req *api.LoginRequest) (*TokenPair, error) {
	user, err := s.orm.User.Query().
		Where(entuser.Email(req.Email)).
		Only(ctx)
	switch {
	case err == nil:
	case ent.IsNotFound(err):
		// 对虚拟哈希执行一次 bcrypt 校验，消除查无邮箱与密码错误之间的耗时侧信道。
		_ = verifyPassword(s.dummyPasswordHash, req.Password)
		slog.WarnContext(ctx, "login attempt with unknown email", "email", req.Email)
		return nil, authInvalidCredentials()
	default:
		return nil, authInternal(err, "querying user by email")
	}

	// 先校验密码再检查账户状态，防止未认证攻击者枚举已停用账户。
	if err := verifyPassword(user.PasswordHash, req.Password); err != nil {
		slog.WarnContext(ctx, "invalid password attempt", "user_id", user.ID)
		return nil, authInvalidCredentials()
	}

	if err := s.validateUser(user); err != nil {
		return nil, err
	}

	var resp *TokenPair
	err = withTx(ctx, s.orm, authInternal, func(tx *ent.Tx) error {
		var err error
		resp, err = s.issueTokenPairTx(ctx, tx, user.ID, "")
		if err != nil {
			return err
		}
		if err := tx.User.UpdateOneID(user.ID).SetLastLoginAt(time.Now()).Exec(ctx); err != nil {
			return authInternal(err, "updating last login time")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// RefreshToken 轮换刷新令牌对；检测到已撤销的刷新令牌被重放时，吊销整个令牌族（family_id）。
func (s *Auth) RefreshToken(ctx context.Context, req *api.RefreshTokenRequest) (*TokenPair, error) {
	tokenHash := hashToken(req.RefreshToken)

	var (
		resp           *TokenPair
		reusedFamilyID string
		reusedUserID   string
	)
	err := withTx(ctx, s.orm, authInternal, func(tx *ent.Tx) error {
		rt, err := tx.RefreshToken.Query().
			Where(refreshtoken.TokenHash(tokenHash)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return refreshTokenError(httperr.PreconditionTokenInvalid, err)
			}
			return authInternal(err, "querying refresh token")
		}

		if rt.IsRevoked {
			reusedFamilyID = rt.FamilyID
			reusedUserID = rt.UserID
			return refreshTokenError(httperr.PreconditionTokenRevoked, nil)
		}

		if !rt.ExpiresAt.After(time.Now()) {
			return refreshTokenError(httperr.PreconditionTokenExpired, nil)
		}

		revoked, err := tx.RefreshToken.Update().
			Where(
				refreshtoken.ID(rt.ID),
				refreshtoken.IsRevoked(false),
			).
			SetIsRevoked(true).
			Save(ctx)
		if err != nil {
			return authInternal(err, "revoking old refresh token")
		}
		if revoked == 0 {
			reusedFamilyID = rt.FamilyID
			reusedUserID = rt.UserID
			return refreshTokenError(httperr.PreconditionTokenRevoked, nil)
		}

		user, err := tx.User.Query().
			Where(entuser.ID(rt.UserID)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return refreshTokenError(httperr.PreconditionTokenInvalid, err)
			}
			return authInternal(err, "querying user for refresh")
		}

		if err := s.validateUser(user); err != nil {
			return err
		}

		resp, err = s.issueTokenPairTx(ctx, tx, user.ID, rt.FamilyID)
		return err
	})
	if err != nil {
		if reusedFamilyID != "" {
			slog.WarnContext(ctx, "refresh token reuse detected; revoking token family",
				"user_id", reusedUserID,
				"family_id", reusedFamilyID,
			)
			if _, revokeErr := s.orm.RefreshToken.Update().
				Where(
					refreshtoken.FamilyID(reusedFamilyID),
					refreshtoken.IsRevoked(false),
				).
				SetIsRevoked(true).
				Save(ctx); revokeErr != nil {
				slog.ErrorContext(ctx, "failed to revoke compromised refresh token family",
					"family_id", reusedFamilyID,
					"error", revokeErr,
				)
			}
		}
		return nil, err
	}

	return resp, nil
}

// Logout 撤销该用户全部未撤销的刷新令牌（已签发的短周期无状态 Access JWT 将自然过期）。
func (s *Auth) Logout(ctx context.Context, userID string) error {
	_, err := s.orm.RefreshToken.Update().
		Where(
			refreshtoken.UserID(userID),
			refreshtoken.IsRevoked(false),
		).
		SetIsRevoked(true).
		Save(ctx)
	if err != nil {
		return authInternal(err, "revoking user refresh tokens on logout")
	}
	return nil
}

// ChangePassword 修改用户密码并在同一事务内撤销该用户全部活跃刷新令牌。
func (s *Auth) ChangePassword(ctx context.Context, userID string, req *api.ChangePasswordRequest) error {
	u, err := s.orm.User.Query().
		Where(entuser.IDEQ(userID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return userNotFound(errAuth, userID)
		}
		return authInternal(err, "fetching user for password change")
	}

	if err := verifyPassword(u.PasswordHash, req.OldPassword); err != nil {
		slog.WarnContext(ctx, "invalid old password", "user_id", userID)
		return errAuth.
			User(userID).
			With(httperr.MetaKey, httperr.PreconditionInfo{
				Type:    httperr.PreconditionInvalidPassword,
				Subject: "old_password",
			}).
			Wrap(httperr.BadRequest)
	}

	newHash, err := hashPassword(req.NewPassword, s.bcryptCost)
	if err != nil {
		return authInternal(err, "hashing new password")
	}

	return withTx(ctx, s.orm, authInternal, func(tx *ent.Tx) error {
		if err := tx.User.UpdateOneID(userID).SetPasswordHash(newHash).Exec(ctx); err != nil {
			return authInternal(err, "updating password")
		}
		if _, err := tx.RefreshToken.Update().
			Where(refreshtoken.UserID(userID), refreshtoken.IsRevoked(false)).
			SetIsRevoked(true).
			Save(ctx); err != nil {
			return authInternal(err, "revoking refresh tokens after password change")
		}
		return nil
	})
}
