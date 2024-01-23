package runtime

import sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

type derivedAddr struct {
	module  string
	kind    string
	address sdkTypes.Address
}

var fromBech = sdkTypes.NewAddressFromBech32

// List of special addresses (aka reserved addresses) that relate to runtimes.
// Not all of these are exposed in the Go client SDK; search runtime-sdk (Rust) for `Address::from_module`.
// The address is derived as sdkTypes.NewAddressForModule(addr.module, []byte(addr.kind)) in the actual runtime;
// here, we keep the address hardcoded for robustness and discoverability.
var (
	accountsCommonPool                 = derivedAddr{"accounts", "common-pool", fromBech("oasis1qz78phkdan64g040cvqvqpwkplfqf6tj6uwcsh30")}
	accountsFeeAccumulator             = derivedAddr{"accounts", "fee-accumulator", fromBech("oasis1qp3r8hgsnphajmfzfuaa8fhjag7e0yt35cjxq0u4")}
	rewardsRewardPool                  = derivedAddr{"rewards", "reward-pool", fromBech("oasis1qp7x0q9qahahhjas0xde8w0v04ctp4pqzu5mhjav")}
	consensusAccountsPendingWithdrawal = derivedAddr{"consensus_accounts", "pending-withdrawal", fromBech("oasis1qr677rv0dcnh7ys4yanlynysvnjtk9gnsyhvm6ln")}
	consensusAccountsPendingDelegation = derivedAddr{"consensus_accounts", "pending-delegation", fromBech("oasis1qzcdegtf7aunxr5n5pw7n5xs3u7cmzlz9gwmq49r")}
)
