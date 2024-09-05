package evm

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer/runtime/encryption"
	"github.com/oasisprotocol/nexus/common"
)

// EVMMaybeUnmarshalEncryptedData breaks down a possibly encrypted data +
// result into their encryption envelope fields. If the data is not encrypted,
// it returns nil with no error.
//
// LESSON: Oasis paratime transaction structure.
//
// Oasis paratime transactions conform to the oasis format transaction payload:
// https://github.com/oasisprotocol/oasis-sdk/blob/04e8918a9b3eb90caffe115ea2d46b5742438714/client-sdk/go/types/transaction.go#L212
//
// Transaction payloads may be encrypted in oasis. To express that, the
// Transaction.Call field of the transaction would have a call format set, e.g.
//     Call.Format: 'encrypted/x25519-deoxysii'
//
// This gives rise to the concept of outer/inner calls for oasis-format
// transactions, where the inner call would be encrypted and stored in the
// outer Call.Body. However, when the runtime evm module converts ethereum
// format transactions to oasis format transaction payloads, they are never
// in this encrypted mode because there is no native ethereum way to express
// an encrypted transaction.
//
// Thus, for ethereum format transactions, the outer oasis format Call.Format
// is always 0/plain (or equivalently, unset). This outer Call directly has
// the method and body of the transaction. For example, the method may be
// "evm.Call" and the body would be the corresponding struct:
// https://github.com/oasisprotocol/oasis-sdk/blob/04e8918a9b3eb90caffe115ea2d46b5742438714/client-sdk/go/modules/evm/types.go#L13
//
// Here is where we may encounter sapphire-style encryption. In the first and
// most basic case, there is no sapphire-style encryption at all and the fields
// of the evm.Call struct are the exact bytes that would go into the evm.
// Note that Sapphire supports this mode.
//
// The result of this transaction is stored in the associated CallResult struct
// that is returned alongside the transaction in TransactionWithResult:
// https://github.com/oasisprotocol/oasis-sdk/blob/main/client-sdk/go/client/client.go#L146
//
// A transaction may also be in sapphire encrypted mode. In this case, the
// evm.Call.Data or evm.Create.Initcode field is itself a cbor-encoded oasis
// format runtime-sdk Call struct that we term the "sapphire outer call". This
// struct is the same oasis format Transaction.Call mentioned above:
// https://github.com/oasisprotocol/oasis-sdk/blob/04e8918a9b3eb90caffe115ea2d46b5742438714/client-sdk/go/types/transaction.go#L363
//
// In encrypted mode, the sapphire outer call has a Call.Format of
// 'encrypted/x25519-deoxysii', and the call.Body is a cbor-encoded encryption
// envelope, which can be decrypted and parsed into another oasis format Call
// struct that we term the "sapphire inner call". Note that Nexus never has
// the keys the decrypt this envelope to get the inner struct since the
// Sapphire transaction is encrypted. The sapphire inner call.Body has the evm
// input data, and the inner call.Format is always unset.
//
// The results of sapphire encrypted transactions are stored similarly. In the
// first case, we should note that sapphire encrypted transactions may have
// unencrypted results if the inputs were not encrypted. Unencrypted transaction
// results are stored in the same associated CallResult as the results of
// unencrypted transactions. We term this CallResult the "outer call result."
//
// Encrypted results can be recovered by cbor-unmarshalling the outer
// CallResult.Ok field into another CallResult struct that we term the
// "sapphire outer call result." Currently, the sapphire outer CallResult.Ok
// stores the result envelope of successful sapphire encrypted transactions.
// However, in older versions of Sapphire and for non-evm transactions, this
// result envelope is stored in the sapphire outer CallResult.Unknown.
//
// The result envelope contains the encrypted "sapphire inner call result",
// where the sapphire inner CallResult.Ok field contains the output data
// from the evm. Note that Nexus does not have the keys to decrypt and access
// this sapphire inner call result.
//
// Side note: The CallResult struct does not specify its own encryption format.
// Rather, it follows the format of its corresponding oasis format Call. The
// "outer call" Call.Format corresponds to the outer call result, and the
// "sapphire outer call" Call.Format corresponds to the sapphire outer call
// result.
//
// Currently, if an encrypted transaction fails, the error is bubbled up to
// the outer CallResult.Failed. However, in older versions of Sapphire the
// error was stored in the sapphire outer CallResult.Failed.
//
// Lastly, there's also a "not-encryption-but-sapphire-something" case where
// the sapphire outer Call.Format is set to "plain". In this case, there's no
// sapphire inner call and the sapphire outer Call.Body has the evm input data.
// Nexus does not extract the evm input data directly here like the way it does
// for unencrypted evm.Call transactions.
//
// The results of these "plain" transactions are returned in the outer
// CallResult.
//
// Outer oasis Call.Body
//   -> inner oasis Call
//   -> evm.Call
//        -> sapphire outer Call.Body
//             -> sapphire inner Call
//
// To summarize:
// * Non-Sapphire
//   * Tx Body: Stored in oasis outer Call.Body
//   * Success Result: Stored in outer oasis CallResult.Ok
//   * Failure Result: Stored in outer oasis CallResult.Failed
// * Sapphire Encrypted
//   * Tx Body: Encryption envelope stored in sapphire outer Call.Body
//   * Success Result: Encryption envelope stored in either sapphire outer
//                     CallResult.Ok (new) or CallResult.Unknown (old)
//   * Failure Result: Stored in either the sapphire outer CallResult.Failed (old)
//                     or the outer oasis CallResult.Failed (new)
// * Sapphire Plain(0)
//   * Tx Body: Stored in the sapphire outer Call.Body
//   * Success Result: Stored in outer oasis CallResult.Ok
//   * Failure Result: Stored in outer oasis CallResult.Failed
func EVMMaybeUnmarshalEncryptedData(data []byte, result *[]byte) (*encryption.EVMEncryptedData, error) {
	var encryptedData encryption.EVMEncryptedData
	var call sdkTypes.Call
	if cbor.Unmarshal(data, &call) != nil {
		// Invalid CBOR means it's bare Ethereum format data. This is normal.
		// https://github.com/oasisprotocol/oasis-sdk/blob/runtime-sdk/v0.3.0/runtime-sdk/modules/evm/src/lib.rs#L626
		return nil, nil
	}
	encryptedData.Format = common.CallFormat(call.Format.String())
	switch call.Format {
	case sdkTypes.CallFormatEncryptedX25519DeoxysII:
		var callEnvelope sdkTypes.CallEnvelopeX25519DeoxysII
		if err := cbor.Unmarshal(call.Body, &callEnvelope); err != nil {
			return nil, fmt.Errorf("outer call format %s unmarshal body: %w", call.Format, err)
		}
		encryptedData.PublicKey = callEnvelope.Pk[:]
		encryptedData.DataNonce = callEnvelope.Nonce[:]
		encryptedData.DataData = callEnvelope.Data
	// Plain txs have no encrypted fields to extract.
	case sdkTypes.CallFormatPlain:
		return nil, nil
	// If you are adding new call formats, remember to add them to the
	// database call_format enum too.
	default:
		return nil, fmt.Errorf("outer call format %s (%d) not supported", call.Format, call.Format)
	}
	var callResult sdkTypes.CallResult
	if result != nil {
		if err := cbor.Unmarshal(*result, &callResult); err != nil {
			return nil, fmt.Errorf("unmarshal outer call result: %w", err)
		}
		switch call.Format {
		case sdkTypes.CallFormatEncryptedX25519DeoxysII:
			switch {
			// The result is encrypted, in which case it is stored in callResult.Unknown
			case callResult.IsUnknown():
				var resultEnvelope sdkTypes.ResultEnvelopeX25519DeoxysII
				if err := cbor.Unmarshal(callResult.Unknown, &resultEnvelope); err != nil {
					return nil, fmt.Errorf("outer call result unmarshal unknown: %w", err)
				}
				encryptedData.ResultNonce = resultEnvelope.Nonce[:]
				encryptedData.ResultData = resultEnvelope.Data
			// The result may be unencrypted, in which case it is stored in callResult.Ok
			//
			// Note: IsUnknown() is already checked above, but we keep it here to make the
			// logic explicit in the event of future refactors. IsSuccess() returns true both
			// when the call succeeded and when it is unknown. However, we want this case
			// to run iff the call succeeded.
			case callResult.IsSuccess() && !callResult.IsUnknown():
				var resultEnvelope sdkTypes.ResultEnvelopeX25519DeoxysII
				if err := cbor.Unmarshal(callResult.Ok, &resultEnvelope); err != nil {
					return nil, fmt.Errorf("outer call result unmarshal ok: %w", err)
				}
				encryptedData.ResultNonce = resultEnvelope.Nonce[:]
				encryptedData.ResultData = resultEnvelope.Data
			// For older Sapphire txs, the outer oasis CallResult may be Ok
			// and the outer sapphire CallResult Failed. In this case, we
			// extract the failed callResult.
			case callResult.Failed != nil:
				encryptedData.FailedCallResult = callResult.Failed
			default:
				return nil, fmt.Errorf("outer call result unsupported variant")
			}
		default:
			// We have already checked when decoding the call envelope,
			// but I'm keeping this default case here so we don't forget
			// if this code gets restructured.
			return nil, fmt.Errorf("outer call result format %s (%d) not supported", call.Format, call.Format)
		}
	}
	return &encryptedData, nil
}
