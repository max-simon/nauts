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
		claims.Permissions = permissionsToNats(permissions)
	}

	// this is to support signing keys
	if issuerAccount != "" {
		claims.IssuerAccount = issuerAccount
	}

	token, err := claims.Encode(signerAdapter{issuerSigner})
	if err != nil {
		return "", fmt.Errorf("encoding user JWT: %w", err)
	}

	return token, nil
}

// permissionsToNats converts policy.NatsPermissions to natsjwt.Permissions.
func permissionsToNats(perms *policy.NatsPermissions) natsjwt.Permissions {
	var natsPerms natsjwt.Permissions

	pubList := perms.PubList()
	if len(pubList) > 0 {
		natsPerms.Pub.Allow = pubList
	}

	subList := perms.SubList()
	if len(subList) > 0 {
		natsPerms.Sub.Allow = subList
	}

	return natsPerms
}

// signerAdapter adapts our Signer interface to nkeys.KeyPair for JWT encoding.
type signerAdapter struct {
	signer Signer
}

func (s signerAdapter) Seed() ([]byte, error) {
	return nil, fmt.Errorf("seed not available")
}

func (s signerAdapter) PublicKey() (string, error) {
	return s.signer.PublicKey(), nil
}

func (s signerAdapter) PrivateKey() ([]byte, error) {
	return nil, fmt.Errorf("private key not available")
}

func (s signerAdapter) Sign(input []byte) ([]byte, error) {
	return s.signer.Sign(input)
}

func (s signerAdapter) Verify(input, sig []byte) error {
	return fmt.Errorf("verify not implemented")
}

func (s signerAdapter) Wipe() {}

func (s signerAdapter) Open(input []byte, sender string) ([]byte, error) {
	return nil, fmt.Errorf("open not implemented")
}

func (s signerAdapter) Seal(input []byte, recipient string) ([]byte, error) {
	return nil, fmt.Errorf("seal not implemented")
}

func (s signerAdapter) SealWithRand(input []byte, recipient string, rr io.Reader) ([]byte, error) {
	return nil, fmt.Errorf("seal with rand not implemented")
}
