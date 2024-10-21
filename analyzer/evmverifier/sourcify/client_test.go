package sourcify_test

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/analyzer/evmverifier/sourcify"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
)

var (
	// Source: https://sourcify.dev/server/files/any/23294/0x127c49aE10e3c18be057106F4d16946E3Ae43975
	//
	//go:embed testdata/get_contract_source_files_response.json
	mockGetContractSourceFilesResponse []byte

	// Source: https://sourcify.dev/server/files/contracts/any/23295
	//
	//go:embed testdata/get_contract_addresses_any_0.json
	mockGetContractAddressesAnyPage0Response []byte

	// Source: https://sourcify.dev/server/files/contracts/any/23295?page=1
	//
	//go:embed testdata/get_contract_addresses_any_1.json
	mockGetContractAddressesAnyPage1Response []byte

	// Source: https://sourcify.dev/server/files/contracts/any/23295
	//
	//go:embed testdata/get_contract_addresses_full_0.json
	mockGetContractAddressesFullPage0Response []byte

	// Source: https://sourcify.dev/server/files/contracts/any/23295?page=1
	//
	//go:embed testdata/get_contract_addresses_full_1.json
	mockGetContractAddressesFullPage1Response []byte

	// Source: https://sourcify.dev/server/files/contracts/any/23295?page=2
	//
	//go:embed testdata/get_contract_addresses_empty_page.json
	mockGetContractAddressesEmptyPageResponse []byte
)

func TestGetVerifiedContractAddresses(t *testing.T) {
	require := require.New(t)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/files/contracts/any"):
			switch {
			case !r.URL.Query().Has("page") || r.URL.Query().Get("page") == "0":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(mockGetContractAddressesAnyPage0Response)
			case r.URL.Query().Get("page") == "1":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(mockGetContractAddressesAnyPage1Response)
			default:
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(mockGetContractAddressesEmptyPageResponse)
			}
			return
		case strings.HasPrefix(r.URL.Path, "/files/contracts/full"):
			switch {
			case !r.URL.Query().Has("page") || r.URL.Query().Get("page") == "0":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(mockGetContractAddressesFullPage0Response)
			case r.URL.Query().Get("page") == "1":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(mockGetContractAddressesFullPage1Response)
			default:
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(mockGetContractAddressesEmptyPageResponse)
			}
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer testServer.Close()

	testClient, err := sourcify.NewClient(testServer.URL, common.ChainNameTestnet, log.NewDefaultLogger("testing/sourcify"))
	require.NoError(err, "NewClient")

	addresses, err := testClient.GetVerifiedContractAddresses(context.Background(), common.RuntimeEmerald)
	require.NoError(err, "GetVerifiedContractAddresses")

	nPartial := 0
	nFull := 0
	for _, level := range addresses {
		switch level {
		case sourcify.VerificationLevelPartial:
			nPartial++
		case sourcify.VerificationLevelFull:
			nFull++
		default:
			require.FailNowf("GetVerifiedContractAddresses", "unexpected verification level %s", level)
		}
	}
	require.Equal(249, nFull)
	require.Equal(42, nPartial)
}

func TestGetContractSourceFiles(t *testing.T) {
	require := require.New(t)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/files/any/42261/0xca2ad74003502af6B727e846Fab40D6cb8Da0035") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockGetContractSourceFilesResponse)
	}))
	defer testServer.Close()

	testClient, err := sourcify.NewClient(testServer.URL, common.ChainNameTestnet, log.NewDefaultLogger("testing/sourcify"))
	require.NoError(err, "NewClient")

	addr := ethCommon.HexToAddress("0xca2ad74003502af6B727e846Fab40D6cb8Da0035")
	sourceFiles, metadata, err := testClient.GetContractSourceFiles(context.Background(), common.RuntimeEmerald, addr)
	require.NoError(err, "GetContractSourceFiles")

	var metadataMap map[string]interface{}
	require.NoError(json.Unmarshal(metadata, &metadataMap), "metadata response unmarshal")

	require.Len(sourceFiles, 3, "GetContractSourceFiles")
	require.Equal(metadataMap["compiler"].(map[string]interface{})["version"], "0.8.18+commit.87f61d96", "Metadata.Compiler.Version")

	// Test non-existing contract.
	_, _, err = testClient.GetContractSourceFiles(context.Background(), common.RuntimeCipher, addr)
	require.Error(err, "GetContractSourceFiles")
}
