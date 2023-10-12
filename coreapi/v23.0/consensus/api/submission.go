package api

import (
	"time"

	"github.com/oasisprotocol/oasis-core/go/common/quantity"
)

const (
	maxSubmissionRetryElapsedTime = 60 * time.Second
	maxSubmissionRetryInterval    = 10 * time.Second
)

// PriceDiscovery is the consensus fee price discovery interface.
// removed interface

type staticPriceDiscovery struct {
	price quantity.Quantity
}

// NewStaticPriceDiscovery creates a price discovery mechanism which always returns the same static
// price specified at construction time.
// removed func

// removed func

type noOpPriceDiscovery struct{}

// removed func

// SubmissionManager is a transaction submission manager interface.
// removed interface
// removed type

// Implements SubmissionManager.
// removed func

// Implements SubmissionManager.
// removed func

// removed func

// removed func

// removed func

// removed func

// Implements SubmissionManager.
// removed func

// Implements SubmissionManager.
// removed func

// NewSubmissionManager creates a new transaction submission manager.
// removed func

// SignAndSubmitTx is a helper function that signs and submits a transaction to
// the consensus backend.
//
// If the nonce is set to zero, it will be automatically filled in based on the
// current consensus state.
//
// If the fee is set to nil, it will be automatically filled in based on gas
// estimation and current gas price discovery.
// removed func

// SignAndSubmitTxWithProof is a helper function that signs and submits
// a transaction to the consensus backend and creates a proof of inclusion.
//
// If the nonce is set to zero, it will be automatically filled in based on the
// current consensus state.
//
// If the fee is set to nil, it will be automatically filled in based on gas
// estimation and current gas price discovery.
// removed func

// NoOpSubmissionManager implements a submission manager that doesn't support submitting transactions.
type NoOpSubmissionManager struct{}

// PriceDiscovery implements SubmissionManager.
// removed func

// EstimateGasAndSetFee implements SubmissionManager.
// removed func

// SignAndSubmitTx implements SubmissionManager.
// removed func

// SignAndSubmitTxWithProof implements SubmissionManager.
// removed func
