package auth

import (
	"errors"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/oops"

	"github.com/liukeshao/go-rest-template/config"
	"github.com/liukeshao/go-rest-template/pkg/httperr"
)

const (
	ReasonTokenExpired = "token_expired"
	ReasonTokenInvalid = "token_invalid"
	ReasonTokenRevoked = "token_revoked"
)

var errAuth = oops.In("auth")

// JWT 负责无状态 HS256 Access Token 的签发与验签，仅携带标准 RegisteredClaims（sub / jti）。
type JWT struct {
	secret       []byte
	issuer       string
	accessExpiry time.Duration
}

func NewJWT(cfg config.JWT) *JWT {
	return &JWT{
		secret:       []byte(cfg.Secret),
		issuer:       cfg.Issuer,
		accessExpiry: cfg.AccessTokenExpiry,
	}
}

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
			return Principal{}, bearerTokenError(ReasonTokenExpired, err)
		}
		return Principal{}, bearerTokenError(ReasonTokenInvalid, err)
	}
	if !parsed.Valid || claims.Subject == "" {
		return Principal{}, bearerTokenError(ReasonTokenInvalid, nil)
	}

	return Principal{
		UserID:  claims.Subject,
		TokenID: claims.ID,
	}, nil
}

func bearerTokenError(reason string, cause error) error {
	b := errAuth.With(httperr.MetaKey, httperr.PreconditionInfo{
		Type:      reason,
		Subject:   "authorization",
		Challenge: "Bearer",
	})
	if cause != nil {
		return b.Join(httperr.Unauthorized, cause)
	}
	return b.Wrap(httperr.Unauthorized)
}
