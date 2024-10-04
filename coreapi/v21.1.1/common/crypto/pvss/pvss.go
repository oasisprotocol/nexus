// Package pvss implements a PVSS backed commit-reveal scheme loosely
// based on the Scalable Randomness Attested by Public Entities
// protocol by Casudo and David.
//
// In practice this implementation omits the things that make SCRAPE
// scalable/fast, and is just a consensus backed PVSS based beacon.
// The differences are as follows:
//
//   - The coding theory based share verification mechanism is not
//     implemented.  The check is as in Schoenmakers' paper.  This
//     could be added at a future date for a performance gain.
//
//   - The commit/reveal based fast path that skips having to recover
//     each participant's secret if they submitted a protocol level
//     reveal is omitted.  It is possible to game the system by
//     publishing shares for one secret and a commitment for another
//     secret, and then choosing to reveal or not after everyone else
//     has revealed.  While this behavior is detectable, it either
//     involves having to recover the secret from the shares anyway
//     rendering the optimization moot, or having a userbase that
//     understands that slashing is integral to the security of the
//     system.
//
// See: https://eprint.iacr.org/2017/216.pdf
package pvss

import "go.dedis.ch/kyber/v3/group/nist"

var (
	// Yes, this uses the NIST P-256 curve, due to vastly superior
	// performance compared to kyber's Ed25519 implementation.  In
	// theory Ed25519 should be faster, but the runtime library's
	// P-256 scalar multiply is optimized, and kyber's Ed25519
	// is basically ref10.
	suite = nist.NewBlakeSHA256P256()
)

// Config is the configuration for an execution of the PVSS protocol.
// removed type

// Instance is an instance of the PVSS protocol.
// removed type

// SetScalar sets the private scalar belonging to an instance.  Under
// most circumstances this will be handled by the constructor.
// removed func

// Commit executes the commit phase of the protocol, generating a commitment
// message to be broadcasted to all participants.
// removed func

// OnCommit processes a commitment message received from a participant.
//
// Note: This assumes that the commit is authentic and attributable.
// removed func

// MayReveal returns true iff it is possible to proceed to the reveal
// step, and the total number of distinct valid commitments received.
// removed func

// Reveal executes the reveal phase of the protocol, generating a reveal
// message to be broadcasted to all participants.
// removed func

// OnReveal processes a reveal message received from a participant.
//
// Note: This assumes that the reveal is authentic and attributable.
// removed func

// MayRecover returns true iff it is possible to proceed to the recovery
// step, and the total number of distinct valid reveals received.
// removed func

// Recover executes the recovery phase of the protocol, returning the resulting
// composite entropy and the indexes of the participants that contributed fully.
// removed func

// New creates a new protocol instance with the provided configuration.
// removed func

// removed func

// removed func

// NewKeyPair creates a new scalar/point pair for use with a PVSS instance.
// removed func

// Commit is a PVSS commit.
type Commit struct {
	Index  int            `json:"index"`
	Shares []*CommitShare `json:"shares"`
}

// Reveal is a PVSS reveal.
type Reveal struct {
	Index           int                  `json:"index"`
	DecryptedShares map[int]*PubVerShare `json:"decrypted_shares"`
}

// CommitState is a PVSS commit and the corresponding decrypted share,
// if any.
type CommitState struct {
	Commit         *Commit      `json:"commit"`
	DecryptedShare *PubVerShare `json:"decrypted_share,omitempty"`
}
