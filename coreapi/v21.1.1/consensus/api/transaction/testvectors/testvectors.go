package testvectors

import (
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"

	"github.com/oasisprotocol/nexus/coreapi/v21.1.1/consensus/api/transaction"
)

const keySeedPrefix = "oasis-core test vectors: "

// TestVector is an Oasis transaction test vector.
type TestVector struct {
	Kind             string                        `json:"kind"`
	SignatureContext string                        `json:"signature_context"`
	Tx               interface{}                   `json:"tx"`
	SignedTx         transaction.SignedTransaction `json:"signed_tx"`
	EncodedTx        []byte                        `json:"encoded_tx"`
	EncodedSignedTx  []byte                        `json:"encoded_signed_tx"`
	// Valid indicates whether the transaction is (statically) valid.
	// NOTE: This means that the transaction passes basic static validation, but
	// it may still not be valid on the given network due to invalid nonce,
	// or due to some specific parameters set on the network.
	Valid            bool                `json:"valid"`
	SignerPrivateKey []byte              `json:"signer_private_key"`
	SignerPublicKey  signature.PublicKey `json:"signer_public_key"`
}

// MakeTestVector generates a new test vector from a transaction.
// removed func

// MakeTestVectorWithSigner generates a new test vector from a transaction using a specific signer.
// removed func
