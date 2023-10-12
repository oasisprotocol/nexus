package api

import (
	"github.com/oasisprotocol/curve25519-voi/primitives/x25519"

	beacon "github.com/oasisprotocol/nexus/coreapi/v23.0/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
)

// minEnclavesPercent is the minimum percentage of key manager enclaves from the key manager
// committee to which the secret needs to be encrypted.
const minEnclavesPercent = 66

// EncryptedMasterSecretSignatureContext is the context used to sign encrypted key manager master secrets.
// removed var statement

// EncryptedEphemeralSecretSignatureContext is the context used to sign encrypted key manager ephemeral secrets.
// removed var statement

// EncryptedSecret is a secret encrypted with Deoxys-II MRAE algorithm.
type EncryptedSecret struct {
	// Checksum is the secret verification checksum.
	Checksum []byte `json:"checksum"`

	// PubKey is the public key used to derive the symmetric key for decryption.
	PubKey x25519.PublicKey `json:"pub_key"`

	// Ciphertexts is the map of REK encrypted secrets.
	Ciphertexts map[x25519.PublicKey][]byte `json:"ciphertexts"`
}

// SanityCheck performs a sanity check on the encrypted secret.
// removed func

// EncryptedMasterSecret is an encrypted master secret.
type EncryptedMasterSecret struct {
	// ID is the runtime ID of the key manager.
	ID common.Namespace `json:"runtime_id"`

	// Generation is the generation of the secret.
	Generation uint64 `json:"generation"`

	// Epoch is the epoch in which the secret was created.
	Epoch beacon.EpochTime `json:"epoch"`

	// Secret is the encrypted secret.
	Secret EncryptedSecret `json:"secret"`
}

// SanityCheck performs a sanity check on the master secret.
// removed func

// EncryptedEphemeralSecret is an encrypted ephemeral secret.
type EncryptedEphemeralSecret struct {
	// ID is the runtime ID of the key manager.
	ID common.Namespace `json:"runtime_id"`

	// Epoch is the epoch to which the secret belongs.
	Epoch beacon.EpochTime `json:"epoch"`

	// Secret is the encrypted secret.
	Secret EncryptedSecret `json:"secret"`
}

// SanityCheck performs a sanity check on the ephemeral secret.
// removed func

// SignedEncryptedMasterSecret is a RAK signed encrypted master secret.
type SignedEncryptedMasterSecret struct {
	// Secret is the encrypted master secret.
	Secret EncryptedMasterSecret `json:"secret"`

	// Signature is a signature of the master secret.
	Signature signature.RawSignature `json:"signature"`
}

// Verify sanity checks the encrypted master secret and verifies its signature.
// removed func

// SignedEncryptedEphemeralSecret is a RAK signed encrypted ephemeral secret.
type SignedEncryptedEphemeralSecret struct {
	// Secret is the encrypted ephemeral secret.
	Secret EncryptedEphemeralSecret `json:"secret"`

	// Signature is a signature of the ephemeral secret.
	Signature signature.RawSignature `json:"signature"`
}

// Verify sanity checks the encrypted ephemeral secret and verifies its signature.
// removed func
