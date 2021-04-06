package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/setting"
	jose "gopkg.in/square/go-jose.v2"

	"github.com/grafana/grafana/pkg/registry"
)

var (
	logger = log.New("jwttoken")
)

func init() {
	registry.Register(&registry.Descriptor{
		Name:         "JWTTokenService",
		Instance:     &JWTTokenImpl{},
		InitPriority: registry.Low,
	})
}

type Service interface {
	HandlePublicKeys(w http.ResponseWriter, r *http.Request)
	NewGrafanaAccessToken(ctx context.Context, user *models.User) (string, error)
}

type JWTTokenImpl struct {
	*JWTToken
	SQLStore *sqlstore.SQLStore `inject:""`
	Cfg      *setting.Cfg       `inject:""`
}

func (i *JWTTokenImpl) Init() error {

	// i.Cfg.AppURL >>

	jt, err := New()
	if err != nil {
		return err
	}
	i.JWTToken = jt
	return nil
}

func New() (*JWTToken, error) {
	algorithm := jose.RS256

	rsaPrivateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}

	keyID := uuid.New().String()
	use := "sig"

	// Instantiate a signer using ECDSA with SHA-384.
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: algorithm, Key: rsaPrivateKey}, &jose.SignerOptions{
		ExtraHeaders: map[jose.HeaderKey]interface{}{
			"kid": keyID,
			"typ": "JWT",
		},
	})
	if err != nil {
		return nil, err
	}

	return &JWTToken{
		keys: Keys{
			SigningKey: &jose.JSONWebKey{
				Key:       rsaPrivateKey,
				KeyID:     keyID,
				Use:       use,
				Algorithm: string(algorithm),
			},
			SigningKeyPub: &jose.JSONWebKey{
				Key:       rsaPrivateKey.Public(),
				KeyID:     keyID,
				Use:       use,
				Algorithm: string(algorithm),
			},
		},
		signer: signer,
	}, nil
}

type JWTToken struct {
	keys   Keys
	signer jose.Signer
}

// Keys hold encryption and signing keys.
// TODO: Should be stored in sql database
type Keys struct {
	// Key for creating and verifying signatures. These may be nil.
	SigningKey    *jose.JSONWebKey
	SigningKeyPub *jose.JSONWebKey

	// Old signing keys which have been rotated but can still be used to validate
	// existing signatures.
	VerificationKeys []VerificationKey

	// The next time the signing key will rotate.
	//
	// For caching purposes, implementations MUST NOT update keys before this time.
	NextRotation time.Time
}

type VerificationKey struct {
	PublicKey *jose.JSONWebKey `json:"publicKey"`
	Expiry    time.Time        `json:"expiry"`
}

func (J *JWTToken) now() time.Time {
	return time.Now()
}

func (j *JWTToken) HandlePublicKeys(w http.ResponseWriter, r *http.Request) {
	if j.keys.SigningKeyPub == nil {
		logger.Error("No public keys found.")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error."))
		return
	}

	jwks := jose.JSONWebKeySet{
		Keys: make([]jose.JSONWebKey, len(j.keys.VerificationKeys)+1),
	}
	jwks.Keys[0] = *j.keys.SigningKeyPub
	for i, verificationKey := range j.keys.VerificationKeys {
		jwks.Keys[i+1] = *verificationKey.PublicKey
	}

	data, err := json.MarshalIndent(jwks, "", "  ")
	if err != nil {
		logger.Error("failed to marshal discovery data: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error."))
		return
	}
	maxAge := j.keys.NextRotation.Sub(j.now())
	if maxAge < (time.Minute * 2) {
		maxAge = time.Minute * 2
	}

	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d, must-revalidate", int(maxAge.Seconds())))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)

}

type audience []string

func (a audience) contains(aud string) bool {
	for _, e := range a {
		if aud == e {
			return true
		}
	}
	return false
}

func (a audience) MarshalJSON() ([]byte, error) {
	if len(a) == 1 {
		return json.Marshal(a[0])
	}
	return json.Marshal([]string(a))
}

type idTokenClaims struct {
	Issuer           string   `json:"iss"`
	Subject          string   `json:"sub"`
	Audience         audience `json:"aud"`
	Expiry           int64    `json:"exp"`
	IssuedAt         int64    `json:"iat"`
	AuthorizingParty string   `json:"azp,omitempty"`
	Nonce            string   `json:"nonce,omitempty"`

	AccessTokenHash string `json:"at_hash,omitempty"`

	Email         string `json:"email,omitempty"`
	EmailVerified *bool  `json:"email_verified,omitempty"`

	Groups []string `json:"groups,omitempty"`

	Name              string `json:"name,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
}

func (j *JWTToken) NewGrafanaAccessToken(ctx context.Context, user *models.User) (string, error) {
	issuedAt := time.Now()

	tok := idTokenClaims{
		Issuer:   "http://localhost:3000",
		Audience: audience([]string{"xxx"}),
		Subject:  user.Login,
		Expiry:   issuedAt.Add(time.Hour).Unix(),
		IssuedAt: issuedAt.Unix(),
	}

	payload, err := json.Marshal(tok)
	if err != nil {
		return "", err
	}

	signature, err := j.signer.Sign(payload)
	if err != nil {
		return "", err
	}

	return signature.CompactSerialize()
}
