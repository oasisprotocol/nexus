package client

import (
	"encoding/json"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/sgx"
	"github.com/oasisprotocol/oasis-core/go/common/sgx/pcs"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"

	"github.com/oasisprotocol/nexus/coreapi/v24.0/common/node"
)

// This file contains custom serialization logic for `rofl.Register` transactions.
// It handles the parsing and enhanced processing of attestation data in rofl.Register transactions.
// The goal is to produce a structured representation of the attestation and embed it into the transaction body.

// Header is a struct matching Oasis Core v24 `pcs.QuoteHeader` interface.
// https://github.com/oasisprotocol/oasis-core/blob/861f035f67089923748f32014810db0f7697fe89/go/common/sgx/pcs/quote.go#L238
type Header struct {
	Version            uint16                 `json:"version"`
	TeeType            pcs.TeeType            `json:"tee_type"`
	QEVendorID         []byte                 `json:"qe_vendor_id"`
	AttestationKeyType pcs.AttestationKeyType `json:"attestation_key_type"`
}

// ReportBody is a struct matching Oasis Core v24 `pcs.ReportBody` interface.
// https://github.com/oasisprotocol/oasis-core/blob/861f035f67089923748f32014810db0f7697fe89/go/common/sgx/pcs/report.go#L11
type ReportBody struct {
	ReportData      []byte              `json:"report_data"`
	EnclaveIdentity sgx.EnclaveIdentity `json:"enclave_identity"`
}

// EnclaveQuote is an Oasis Core v24 `pcs.Quote` with exposed header and report body fields.
// https://github.com/oasisprotocol/oasis-core/blob/861f035f67089923748f32014810db0f7697fe89/go/common/sgx/pcs/quote.go#L45
type EnclaveQuote struct {
	Header     Header     `json:"header"`
	ReportBody ReportBody `json:"report_body"`
	// Signature  pcs.QuoteSignature `json:"signature"` // The interfaces doesn't expose any useful methods for the signature.
}

// ParsedPSCQuoteBundle is an Oasis Core v24 `pcs.QuoteBundle` with parsed TCB and enclave quote.
// https://github.com/oasisprotocol/oasis-core/blob/861f035f67089923748f32014810db0f7697fe89/go/common/sgx/pcs/pcs.go#L41
type ParsedPSCQuoteBundle struct {
	TCB   pcs.TCBBundle `json:"tcb"`
	Quote EnclaveQuote  `json:"quote"`
}

// ParsedPCSQuote is an Oasis Core v24 `quote.Quote` with parsed PCS quote bundle.
// https://github.com/oasisprotocol/oasis-core/blob/861f035f67089923748f32014810db0f7697fe89/go/common/sgx/quote/quote.go#L13
type ParsedPCSQuote struct {
	PCS *ParsedPSCQuoteBundle `json:"pcs"`
}

// ParsedAttestation is an Oasis Core v24 `node.SGXAttestation` with parsed PCS quote.
// https://github.com/oasisprotocol/oasis-core/blob/861f035f67089923748f32014810db0f7697fe89/go/common/node/sgx.go#L138
type ParsedAttestation struct {
	// Quote is the parsed quote.
	Quote ParsedPCSQuote `json:"quote"`

	// Height is the runtime's view of the consensus layer height at the time of attestation.
	Height uint64 `json:"height"`

	// Signature is the signature of the attestation by the enclave (RAK).
	Signature signature.RawSignature `json:"signature"`
}

// Serialize the transaction body with enhanced attestation parsing for SGX hardware.
// If the CapabilityTEE's hardware type is SGX, attempts to parse the attestation field,
// replacing it with a structured SGXAttestation. If parsing fails or the hardware type
// is not SGX, the original transaction body is returned unchanged.
func extractPCSQuote(untyped *map[string]interface{}) (*map[string]interface{}, error) {
	raw, err := json.Marshal(untyped)
	if err != nil {
		return untyped, fmt.Errorf("failed to marshal untyped body: %w", err)
	}
	var body rofl.Register
	if err := json.Unmarshal(raw, &body); err != nil {
		return untyped, fmt.Errorf("failed to unmarshal rofl.Register body: %w", err)
	}

	// If not SGX attestation, return original.
	if uint8(body.EndorsedCapability.CapabilityTEE.Hardware) != uint8(node.TEEHardwareIntelSGX) {
		return untyped, fmt.Errorf("not an SGX attestation")
	}

	// Try parsing the SGX Attestation.
	var sa node.SGXAttestation
	if err := cbor.Unmarshal(body.EndorsedCapability.CapabilityTEE.Attestation, &sa); err != nil {
		return untyped, fmt.Errorf("failed to unmarshal SGX attestation: %w", err)
	}

	// Try parsing the PCS Quote. (We don't try parsing the IAS quote since it's deprecated, so we mostly care for PCS).
	var pcsQuote pcs.Quote
	if sa.Quote.PCS == nil {
		return untyped, fmt.Errorf("missing PCS quote")
	}
	if err := pcsQuote.UnmarshalBinary(sa.Quote.PCS.Quote); err != nil {
		return untyped, fmt.Errorf("failed to unmarshal PCS quote: %w", err)
	}

	// Pick ReportBody from the PCS quote (TODO: add method to oasis-core to expose this).
	val := reflect.ValueOf(&pcsQuote).Elem()
	field := val.FieldByName("reportBody")
	if !field.IsValid() {
		return untyped, fmt.Errorf("failed to extract PCS report body")
	}
	fieldValue := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	reportBody, ok := fieldValue.Interface().(pcs.ReportBody)
	if !ok {
		return untyped, fmt.Errorf("failed to extract PCS report body interface")
	}

	// Update the body with the parsed data.
	parsedRegister := struct {
		rofl.Register
		// Override Attestation field.
		EndorsedCapability struct {
			CapabilityTEE struct {
				node.CapabilityTEE
				Attestation ParsedAttestation `json:"attestation"`
			} `json:"capability_tee"`
			NodeEndorsement signature.Signature `json:"node_endorsement"`
		} `json:"ect"` //nolint: misspell
	}{
		Register: body,
	}
	parsedRegister.EndorsedCapability.CapabilityTEE.Attestation = ParsedAttestation{
		Quote: ParsedPCSQuote{
			PCS: &ParsedPSCQuoteBundle{
				TCB: sa.Quote.PCS.TCB, // PCS is not null, otherwise we wouldn't have reached this point.
				Quote: EnclaveQuote{
					Header: Header{
						Version:            pcsQuote.Header().Version(),
						TeeType:            pcsQuote.Header().TeeType(),
						QEVendorID:         pcsQuote.Header().QEVendorID(),
						AttestationKeyType: pcsQuote.Header().AttestationKeyType(),
					},
					ReportBody: ReportBody{
						ReportData:      reportBody.ReportData(),
						EnclaveIdentity: reportBody.AsEnclaveIdentity(),
					},
				},
			},
		},
		Height:    sa.Height,
		Signature: sa.Signature,
	}

	raw, err = json.Marshal(parsedRegister)
	if err != nil {
		return untyped, fmt.Errorf("failed to marshal parsed body: %w", err)
	}
	var newrt *map[string]interface{}
	if err := json.Unmarshal(raw, &newrt); err != nil {
		return untyped, fmt.Errorf("failed to unmarshal parsed body: %w", err)
	}
	return newrt, nil
}
