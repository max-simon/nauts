// Package jwt provides JWT issuance and signing for NATS entities.
package jwt

// Signer provides cryptographic signing capabilities.
type Signer interface {
	// PublicKey returns the public key associated with this signer.
	PublicKey() string

	// Sign signs the given data and returns the signature.
	Sign(data []byte) ([]byte, error)
}
