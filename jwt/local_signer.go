package jwt

import (
	"fmt"

	"github.com/nats-io/nkeys"
)

// LocalSigner implements the Signer interface using a local private key.
type LocalSigner struct {
	publicKey string
	keyPair   nkeys.KeyPair
}

// NewLocalSigner creates a new LocalSigner from a seed (private key).
// The seed should be an nkey seed string (e.g., "SOABC...").
func NewLocalSigner(seed string) (*LocalSigner, error) {
	kp, err := nkeys.FromSeed([]byte(seed))
	if err != nil {
		return nil, fmt.Errorf("parsing seed: %w", err)
	}

	pub, err := kp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("getting public key: %w", err)
	}

	return &LocalSigner{
		publicKey: pub,
		keyPair:   kp,
	}, nil
}

// PublicKey returns the public key associated with this signer.
func (s *LocalSigner) PublicKey() string {
	return s.publicKey
}

// Sign signs the given data and returns the signature.
func (s *LocalSigner) Sign(data []byte) ([]byte, error) {
	sig, err := s.keyPair.Sign(data)
	if err != nil {
		return nil, fmt.Errorf("signing data: %w", err)
	}
	return sig, nil
}
