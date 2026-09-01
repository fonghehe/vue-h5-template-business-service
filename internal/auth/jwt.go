// Package auth issues and verifies the JWTs shared by the vue-h5-template
// backend. The AI service validates the same access tokens, so the secret,
// issuer and audience must be configured identically on both services.
//
// Two token types are minted:
//
//   - access tokens, short lived, sent as "Authorization: Bearer <token>";
//   - refresh tokens, long lived, delivered as an HttpOnly cookie scoped to
//     the auth paths so that JavaScript can never read them.
package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType distinguishes access from refresh tokens in the typ claim.
type TokenType string

const (
	// AccessToken is presented on every authenticated API call.
	AccessToken TokenType = "access"
	// RefreshToken is only accepted by the refresh endpoint.
	RefreshToken TokenType = "refresh"
)

// ErrInvalidToken is returned for any token that fails verification. The
// specific reason is intentionally not exposed to callers.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims is the claim set carried by both token types.
type Claims struct {
	// Username is the login name, echoed back to clients for convenience.
	Username string `json:"username,omitempty"`
	// RealName is the display name.
	RealName string `json:"realName,omitempty"`
	// Roles carries the authorisation roles granted to the subject.
	Roles []string `json:"roles,omitempty"`
	// Type is either access or refresh.
	Type TokenType `json:"typ,omitempty"`
	jwt.RegisteredClaims
}

// Issuer mints and verifies tokens.
type Issuer struct {
	secret   []byte
	issuer   string
	audience string
}

// New builds an Issuer from configuration.
func New(secret, issuer, audience string) *Issuer {
	return &Issuer{secret: []byte(secret), issuer: issuer, audience: audience}
}

// TokenPair is the result of a successful login or refresh.
type TokenPair struct {
	AccessToken string
	// RefreshToken is empty when the caller only rotated an access token.
	RefreshToken string
	// ExpiresIn is the access token lifetime in seconds.
	ExpiresIn int64
}

// Issue mints an access token and, when refresh is requested, a refresh token.
func (i *Issuer) Issue(userID uint, username, realName string, roles []string, accessTTL, refreshTTL time.Duration, withRefresh bool) (TokenPair, error) {
	now := time.Now()
	access, err := i.sign(Claims{
		Username: username,
		RealName: realName,
		Roles:    roles,
		Type:     AccessToken,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    i.issuer,
			Subject:   strconv.FormatUint(uint64(userID), 10),
			Audience:  jwt.ClaimStrings{i.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	})
	if err != nil {
		return TokenPair{}, err
	}

	pair := TokenPair{AccessToken: access, ExpiresIn: int64(accessTTL.Seconds())}
	if withRefresh {
		refresh, err := i.sign(Claims{
			Username: username,
			Type:     RefreshToken,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    i.issuer,
				Subject:   strconv.FormatUint(uint64(userID), 10),
				Audience:  jwt.ClaimStrings{i.audience},
				ExpiresAt: jwt.NewNumericDate(now.Add(refreshTTL)),
				IssuedAt:  jwt.NewNumericDate(now),
				NotBefore: jwt.NewNumericDate(now),
				ID:        uuid.NewString(),
			},
		})
		if err != nil {
			return TokenPair{}, err
		}
		pair.RefreshToken = refresh
	}
	return pair, nil
}

func (i *Issuer) sign(claims Claims) (string, error) {
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Verify parses and validates a token, enforcing signature, issuer, audience,
// expiry and the expected token type.
func (i *Issuer) Verify(token string, expected TokenType) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return i.secret, nil
	},
		jwt.WithIssuer(i.issuer),
		jwt.WithAudience(i.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	if claims.Type != expected {
		return nil, ErrInvalidToken
	}
	if claims.Subject == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// UserID extracts the numeric subject from verified claims.
func (c *Claims) UserID() (uint, error) {
	value, err := strconv.ParseUint(c.Subject, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse token subject %q: %w", c.Subject, err)
	}
	return uint(value), nil
}

// HasRole reports whether the claims carry the given role.
func (c *Claims) HasRole(role string) bool {
	for _, candidate := range c.Roles {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(role)) == 1 {
			return true
		}
	}
	return false
}

// Bearer extracts a bearer token from a raw Authorization header value.
func Bearer(header string) (string, bool) {
	scheme, value, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}
