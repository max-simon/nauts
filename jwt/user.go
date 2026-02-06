package jwt

import (
	"fmt"
	"io"
	"time"

	natsjwt "github.com/nats-io/jwt/v2"

	"github.com/msimon/nauts/policy"
)

// IssueUserJWT creates and signs a NATS user JWT.
// Parameters:
//   - userName: the name of the user (for display purposes)
//   - userPublicKey: the public key of the user (subject of the JWT)
//   - ttl: time-to-live for the JWT
//   - permissions: NATS permissions to include in the JWT
//   - issuerSigner: the account signer that issues the JWT
//   - audienceAccount: the public key of the target account (for non-operator mode)
//
// Returns the signed JWT string.
func IssueUserJWT(userName string, userPublicKey string, ttl time.Duration, permissions *policy.NatsPermissions, issuerSigner Signer, audienceAccount string, issuerAccount string) (string, error) {
	claims := natsjwt.NewUserClaims(userPublicKey)
	claims.Name = userName
	// Set audience to the target account's public key (required for non-operator mode)
	if audienceAccount != "" {
		claims.Audience = audienceAccount
	}

	if ttl > 0 {
		claims.Expires = time.Now().Add(ttl).Unix()
	}

	if permissions != nil {
		claims.Permissions = permissions.ToNatsJWT()
	}

	// this is to support signing keys
	if issuerAccount != "" {
		claims.IssuerAccount = issuerAccount
	}

	token, err := claims.Encode(NewSignerAdapter(issuerSigner))
	if err != nil {
		return "", fmt.Errorf("encoding user JWT: %w", err)
	}

	return token, nil
}

// SignerAdapter adapts a Signer interface to nkeys.KeyPair for JWT encoding.
// This allows using our Signer interface with the nats-io/jwt library.
type SignerAdapter struct {
	signer Signer
}

// NewSignerAdapter creates a new SignerAdapter from a Signer.
func NewSignerAdapter(s Signer) SignerAdapter {
	return SignerAdapter{signer: s}
}

func (s SignerAdapter) Seed() ([]byte, error) {
	return nil, fmt.Errorf("seed not available")
}

func (s SignerAdapter) PublicKey() (string, error) {
	return s.signer.PublicKey(), nil
}

func (s SignerAdapter) PrivateKey() ([]byte, error) {
	return nil, fmt.Errorf("private key not available")
}

func (s SignerAdapter) Sign(input []byte) ([]byte, error) {
	return s.signer.Sign(input)
}

func (s SignerAdapter) Verify(input, sig []byte) error {
	return fmt.Errorf("verify not implemented")
}

func (s SignerAdapter) Wipe() {}

func (s SignerAdapter) Open(input []byte, sender string) ([]byte, error) {
	return nil, fmt.Errorf("open not implemented")
}

func (s SignerAdapter) Seal(input []byte, recipient string) ([]byte, error) {
	return nil, fmt.Errorf("seal not implemented")
}

func (s SignerAdapter) SealWithRand(input []byte, recipient string, rr io.Reader) ([]byte, error) {
	return nil, fmt.Errorf("seal with rand not implemented")
}
