package auth

import (
	"errors"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/httperr"
)

var errAuth = oops.In("auth")

// JWT 负责无状态 Access Token（HS256 JWT）的签发与解析验证。
// 仅在标准 RegisteredClaims 中携带 sub（用户 ID）与 jti（令牌 ID），不混入业务资料字段。
type JWT struct {
	secret       []byte
	issuer       string
	accessExpiry time.Duration
}

// NewJWT 根据配置构造无状态 Access JWT 管理器。
func NewJWT(cfg config.JWT) *JWT {
	return &JWT{
		secret:       []byte(cfg.Secret),
		issuer:       cfg.Issuer,
		accessExpiry: cfg.AccessTokenExpiry,
	}
}

// SignAccessToken 为指定 userID 签发短生命周期的无状态 Access JWT。
func (j *JWT) SignAccessToken(userID string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(j.accessExpiry)

	claims := jwt.RegisteredClaims{
		ID:        uuid.NewV7().String(),
		Subject:   userID,
		Issuer:    j.issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, errAuth.With("op", "signing access token").Join(httperr.Internal, err)
	}
	return signed, expiresAt, nil
}

// ParseAccessToken 验证 Access JWT 的签名、算法、签发者与有效期，并返回不可变 Principal。
func (j *JWT) ParseAccessToken(tokenString string) (Principal, error) {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	}
	if j.issuer != "" {
		opts = append(opts, jwt.WithIssuer(j.issuer))
	}

	var claims jwt.RegisteredClaims
	parsed, err := jwt.ParseWithClaims(tokenString, &claims, func(_ *jwt.Token) (any, error) {
		return j.secret, nil
	}, opts...)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return Principal{}, bearerTokenError(httperr.PreconditionTokenExpired, err)
		}
		return Principal{}, bearerTokenError(httperr.PreconditionTokenInvalid, err)
	}
	if !parsed.Valid || claims.Subject == "" {
		return Principal{}, bearerTokenError(httperr.PreconditionTokenInvalid, nil)
	}

	return Principal{
		UserID:  claims.Subject,
		TokenID: claims.ID,
	}, nil
}

func bearerTokenError(reason string, cause error) error {
	b := errAuth.With(httperr.MetaKey, httperr.PreconditionInfo{
		Type:    reason,
		Subject: "authorization",
		Detail:  "Bearer",
	})
	if cause != nil {
		return b.Join(httperr.Unauthorized, cause)
	}
	return b.Wrap(httperr.Unauthorized)
}
