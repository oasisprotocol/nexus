package runtime

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer/runtime/encryption"
	"github.com/oasisprotocol/nexus/common"
)

// OasisMaybeUnmarshalEncryptedData breaks down a possibly encrypted call +
// result into their encryption envelope fields. If the call is not encrypted,
// it returns nil with no error.
func OasisMaybeUnmarshalEncryptedData(call *sdkTypes.Call, callResult *sdkTypes.CallResult) (*encryption.EncryptedData, error) {
	var encryptedData encryption.EncryptedData
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
	if callResult != nil {
		switch call.Format {
		case sdkTypes.CallFormatEncryptedX25519DeoxysII:
			var resultEnvelope sdkTypes.ResultEnvelopeX25519DeoxysII
			if callResult.IsUnknown() {
				// The result is encrypted, in which case it is stored in callResult.Unknown
				if err := cbor.Unmarshal(callResult.Unknown, &resultEnvelope); err != nil {
					return nil, fmt.Errorf("outer call result unmarshal unknown: %w", err)
				}
				encryptedData.ResultNonce = resultEnvelope.Nonce[:]
				encryptedData.ResultData = resultEnvelope.Data
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
