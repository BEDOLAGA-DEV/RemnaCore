package authutil

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	JWTIssuerName = "remnacore"
)

// UserClaims carries the application-level identity embedded in each JWT.
type UserClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

// jwtCustomClaims combines UserClaims with the standard registered claims.
type jwtCustomClaims struct {
	UserClaims
	jwt.RegisteredClaims
}

// JWTIssuer creates and validates ES256 JWTs.
type JWTIssuer struct {
	privateKey *ecdsa.PrivateKey
	publicKey  *ecdsa.PublicKey
}

// NewJWTIssuer returns a JWTIssuer configured with the given ECDSA key pair.
// Either key may be nil if only signing or only verification is needed.
func NewJWTIssuer(privateKey *ecdsa.PrivateKey, publicKey *ecdsa.PublicKey) *JWTIssuer {
	return &JWTIssuer{
		privateKey: privateKey,
		publicKey:  publicKey,
	}
}

// Sign produces an ES256 JWT containing the given UserClaims and standard
// registered claims (iat, exp, jti, iss). The token expires after ttl.
func (j *JWTIssuer) Sign(claims UserClaims, ttl time.Duration) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwtCustomClaims{
		UserClaims: claims,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    JWTIssuerName,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        uuid.Must(uuid.NewV7()).String(),
		},
	})

	signed, err := token.SignedString(j.privateKey)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}
	return signed, nil
}

// Verify parses and validates the token string, ensuring the signing method is
// ECDSA and the token has not expired. On success it returns the embedded
// UserClaims.
func (j *JWTIssuer) Verify(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwtCustomClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	custom, ok := token.Claims.(*jwtCustomClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return &custom.UserClaims, nil
}
