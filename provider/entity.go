package provider

import "github.com/msimon/nauts/jwt"

// Operator represents a NATS operator entity.
type Operator struct {
	name      string
	publicKey string
	signer    jwt.Signer
}

// Name returns the operator's name.
func (o *Operator) Name() string {
	return o.name
}

// PublicKey returns the operator's public key.
func (o *Operator) PublicKey() string {
	return o.publicKey
}

// Signer returns the signer for this operator.
func (o *Operator) Signer() jwt.Signer {
	return o.signer
}

// Account represents a NATS account entity.
type Account struct {
	name      string
	publicKey string
	signer    jwt.Signer
}

// Name returns the account's name.
func (a *Account) Name() string {
	return a.name
}

// PublicKey returns the account's public key.
func (a *Account) PublicKey() string {
	return a.publicKey
}

// Signer returns the signer for this account.
func (a *Account) Signer() jwt.Signer {
	return a.signer
}
