package consensus

import (
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"

	roothashCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/roothash/api"
	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api/transaction"
	roothashDamask "github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api"
)

func TestCobaltTx(t *testing.T) {
	// Setup Cobalt-only roothash.ExecutorCommit transaction
	bodyBytes, err := hex.DecodeString("a26269645820000000000000000000000000000000000000000000000000000000000000ff0367636f6d6d69747381a2697369676e6174757265a2697369676e61747572655840c6974b7385427e41fea391e67e7795605cd1b9532bbe08acf140ea4cd9548fe9b51c7b7f5bd3e4503fe3c99d1fd91d3c54837f1640382dd86b774778dc8d69096a7075626c69635f6b6579582014de3f20514009d1aa9e9fa144381a5342159d19c82cd53ded21723d7044a64d73756e747275737465645f7261775f76616c75655903d8a666686561646572a565726f756e6419ab3567696f5f726f6f745820f25e088a5f40ee6d26c527bcc60781fd7f655b829e302dc7f68ca3a14f853cc76a73746174655f726f6f7458205e6ae2fc0992eb20bca5457e57bf5e9a3cc414cfc6b40b4f92664dcbee3c95096d6d657373616765735f686173685820c672b8d1ef56ed28ab87c3622c5114069bdd3ad7b8f9737498d0c01ecef0967a6d70726576696f75735f686173685820806469c247e30fb9d2bf7c54a238a195495d389886915803be3dbf64b99b09826772616b5f7369675840000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000006a696e7075745f726f6f74582099265585e51182de59a17ea7333d31c28580652ab7c8af8f69ba86c6955b916d6d74786e5f73636865645f736967a2697369676e6174757265584018494171c4d91da9ba1463b3620500d2eb87690a672aea8dd8d9d156a944fc995a96ed8152d727db3358bad284142642f311480309893c726b7c781db52dc90c6a7075626c69635f6b6579582014de3f20514009d1aa9e9fa144381a5342159d19c82cd53ded21723d7044a64d72696e7075745f73746f726167655f7369677382a2697369676e61747572655840ff6183073ee6811a56cd4633ae936e2acec02696710fa5b145b09834c82bb95856960905da53819d6b035b5dec3e407dd3afad73d970bb93217c1d72a68ff8086a7075626c69635f6b65795820029270158d7ad16ef25a7f26bacfd65e25e4f6ca589b6fdaaf669fb40792312fa2697369676e6174757265584076b5d3152ad353d6c1762dcc524ebfde532fa0423bbf75d88a2bd7f7620aae4f33cd067e37f905b55f29f0f60408f542ca3e831e89c8d493efbd1993feb0d2026a7075626c69635f6b657958206dbcf2330a17e0fc52126cca2c2173b5b6530a17a2006a8339de19fe4a47f75c7273746f726167655f7369676e61747572657382a2697369676e61747572655840ab27361e360ca3911ffcc53df832aaf22833cf05200afbebdb2061adedd5afdb562fa0798492743c4ac39d93486646d9343412daece2c071857f42793f7c69016a7075626c69635f6b65795820029270158d7ad16ef25a7f26bacfd65e25e4f6ca589b6fdaaf669fb40792312fa2697369676e6174757265584020e853374fe0065a9161b43bb39f8ee6f0114d1787b6e8f6d104fde3feab0d530944d5805b4396f6353a799bd1f437f8ab44d9c85f04bef1c3bb9e17262579046a7075626c69635f6b657958206dbcf2330a17e0fc52126cca2c2173b5b6530a17a2006a8339de19fe4a47f75c")
	require.Nil(t, err)
	var body cbor.RawMessage
	err = body.UnmarshalCBOR(bodyBytes)
	require.Nil(t, err)
	// Round: 3028327, tx_index: 2
	tx := transaction.Transaction{
		Nonce:  0,
		Method: "roothash.ExecutorCommit",
		Body:   body,
	}
	expected := []byte(`{"id":"000000000000000000000000000000000000000000000000000000000000ff03","commits":[{"untrusted_raw_value":"pmZoZWFkZXKlZXJvdW5kGas1Z2lvX3Jvb3RYIPJeCIpfQO5tJsUnvMYHgf1/ZVuCnjAtx/aMo6FPhTzHanN0YXRlX3Jvb3RYIF5q4vwJkusgvKVFfle/Xpo8xBTPxrQLT5JmTcvuPJUJbW1lc3NhZ2VzX2hhc2hYIMZyuNHvVu0oq4fDYixRFAab3TrXuPlzdJjQwB7O8JZ6bXByZXZpb3VzX2hhc2hYIIBkacJH4w+50r98VKI4oZVJXTiYhpFYA749v2S5mwmCZ3Jha19zaWdYQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABqaW5wdXRfcm9vdFggmSZVheURgt5ZoX6nMz0xwoWAZSq3yK+PabqGxpVbkW1tdHhuX3NjaGVkX3NpZ6Jpc2lnbmF0dXJlWEAYSUFxxNkdqboUY7NiBQDS64dpCmcq6o3Y2dFWqUT8mVqW7YFS1yfbM1i60oQUJkLzEUgDCYk8cmt8eB21LckManB1YmxpY19rZXlYIBTePyBRQAnRqp6foUQ4GlNCFZ0ZyCzVPe0hcj1wRKZNcmlucHV0X3N0b3JhZ2Vfc2lnc4KiaXNpZ25hdHVyZVhA/2GDBz7mgRpWzUYzrpNuKs7AJpZxD6WxRbCYNMgruVhWlgkF2lOBnWsDW13sPkB906+tc9lwu5MhfB1ypo/4CGpwdWJsaWNfa2V5WCACknAVjXrRbvJafya6z9ZeJeT2ylibb9qvZp+0B5IxL6Jpc2lnbmF0dXJlWEB2tdMVKtNT1sF2LcxSTr/eUy+gQju/ddiKK9f3YgquTzPNBn43+QW1Xynw9gQI9ULKPoMeicjUk++9GZP+sNICanB1YmxpY19rZXlYIG288jMKF+D8UhJsyiwhc7W2UwoXogBqgzneGf5KR/dccnN0b3JhZ2Vfc2lnbmF0dXJlc4KiaXNpZ25hdHVyZVhAqyc2HjYMo5Ef/MU9+DKq8igzzwUgCvvr2yBhre3Vr9tWL6B5hJJ0PErDnZNIZkbZNDQS2uziwHGFf0J5P3xpAWpwdWJsaWNfa2V5WCACknAVjXrRbvJafya6z9ZeJeT2ylibb9qvZp+0B5IxL6Jpc2lnbmF0dXJlWEAg6FM3T+AGWpFhtDuzn47m8BFNF4e26PbRBP3j/qsNUwlE1YBbQ5b2NTp5m9H0N/irRNnIXwS+8cO7nhcmJXkEanB1YmxpY19rZXlYIG288jMKF+D8UhJsyiwhc7W2UwoXogBqgzneGf5KR/dc","signature":{"public_key":"FN4/IFFACdGqnp+hRDgaU0IVnRnILNU97SFyPXBEpk0=","signature":"xpdLc4VCfkH+o5HmfneVYFzRuVMrvgis8UDqTNlUj+m1HHt/W9PkUD/jyZ0f2R08VIN/FkA4Ldhrd0d43I1pCQ=="}}]}`)

	// Parse and validate
	parsed, err := unpackTxBody(&tx)
	require.Nil(t, err)
	require.Equal(t, reflect.TypeOf(&roothashCobalt.ExecutorCommit{}).String(), reflect.TypeOf(parsed).String())
	parsedJson, err := json.Marshal(parsed)
	require.Nil(t, err)
	require.Equal(t, expected, parsedJson)
}

func TestDamaskTx(t *testing.T) {
	// Setup Damask-only roothash.ExecutorCommit transaction
	bodyBytes, err := hex.DecodeString("a26269645820000000000000000000000000000000000000000000000000e199119c992377cb67636f6d6d69747381a3637369675840bd1dec77999d5c30b6ed748467c998fd837db67e6252daf9a7b76c56f8df41ae6feffbc12f9285d595b40d6c83e5e979075da4b272da21bf2e023c490cb75f0666686561646572a365726f756e641952d3676661696c757265016d70726576696f75735f68617368582067e13bd5666486c4a050a9caeb8d17fc9728110581426398312b25ce2b3abe20676e6f64655f696458207e7dcb588bccb5aaf2e77029935b2b0c1fc8f52c636e961ff53b766c7dd091fe")
	require.Nil(t, err)
	var body cbor.RawMessage
	err = body.UnmarshalCBOR(bodyBytes)
	require.Nil(t, err)
	// Round: 11453160, tx_index: 1
	tx := transaction.Transaction{
		Nonce:  0,
		Method: "roothash.ExecutorCommit",
		Body:   body,
	}
	expected := []byte(`{"id":"000000000000000000000000000000000000000000000000e199119c992377cb","commits":[{"node_id":"fn3LWIvMtary53Apk1srDB/I9SxjbpYf9Tt2bH3Qkf4=","header":{"round":21203,"previous_hash":"67e13bd5666486c4a050a9caeb8d17fc9728110581426398312b25ce2b3abe20","failure":1},"sig":"vR3sd5mdXDC27XSEZ8mY/YN9tn5iUtr5p7dsVvjfQa5v7/vBL5KF1ZW0DWyD5el5B12ksnLaIb8uAjxJDLdfBg=="}]}`)

	// Parse and validate
	parsed, err := unpackTxBody(&tx)
	require.Nil(t, err)
	require.Equal(t, reflect.TypeOf(&roothashDamask.ExecutorCommit{}).String(), reflect.TypeOf(parsed).String())
	parsedJson, err := json.Marshal(parsed)
	require.Nil(t, err)
	require.Equal(t, expected, parsedJson)
}

func TestMalformedTxBody(t *testing.T) {
	// Setup malformed transaction
	bodyBytes, err := hex.DecodeString("deadbeef")
	require.Nil(t, err)
	var body cbor.RawMessage
	err = body.UnmarshalCBOR(bodyBytes)
	require.Nil(t, err)
	tx := transaction.Transaction{
		Nonce:  0,
		Method: "staking.Allow",
		Body:   body,
	}

	parsed, err := unpackTxBody(&tx)
	require.Nil(t, parsed)
	require.NotNil(t, err)
	require.Equal(t, "unable to cbor-decode consensus tx body: cbor: invalid additional information 30 for type tag, method: staking.Allow, body: deadbeef", err.Error())
}

func TestMalformedTxMethod(t *testing.T) {
	// Setup malformed transaction
	bodyBytes, err := hex.DecodeString("deadbeef")
	require.Nil(t, err)
	var body cbor.RawMessage
	err = body.UnmarshalCBOR(bodyBytes)
	require.Nil(t, err)
	tx := transaction.Transaction{
		Nonce:  0,
		Method: "not_a_valid_method",
		Body:   body,
	}

	parsed, err := unpackTxBody(&tx)
	require.Nil(t, parsed)
	require.NotNil(t, err)
	require.Equal(t, "unable to cbor-decode consensus tx body: unknown tx method, method: not_a_valid_method, body: deadbeef", err.Error())
}
