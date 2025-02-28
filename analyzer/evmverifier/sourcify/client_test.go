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
	// Source: https://sourcify.dev/server/v2/contract/23295/0x8259824f2379ef1473a022cBE074076dF8B88444?fields=sources,metadata
	//
	//go:embed testdata/get_contract_source_files_response.json
	mockGetContractSourceFilesResponse []byte

	// Source: https://sourcify.dev/server/v2/contracts/23295?sort=desc&limit=200
	// Contract 0x229774C75b766C17e8B16B9F3dc891448f40A47B was edited from "exact_match" to "match",
	// so that we test partial matches as well.
	//
	//go:embed testdata/get_contracts.json
	mockGetContracts0Response []byte

	// Source: https://sourcify.dev/server/v2/contracts/23295?sort=desc&limit=200&afterMatchId=6161287
	//
	//go:embed testdata/get_contracts_after_6161287.json
	mockGetContractsAfter6161287Response []byte

	// Source: https://sourcify.dev/server/v2/contracts/23295?sort=desc&limit=200&afterMatchId=1
	//
	//go:embed testdata/get_contracts_empty.json
	mockGetContractsEmptyResponse []byte
)

func TestGetVerifiedContractAddresses(t *testing.T) {
	require := require.New(t)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/v2/contracts/"):
			switch {
			case !r.URL.Query().Has("afterMatchId"):
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(mockGetContracts0Response)
			case r.URL.Query().Get("afterMatchId") == "6161287":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(mockGetContractsAfter6161287Response)
			default:
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(mockGetContractsEmptyResponse)
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
	require.Equal(399, nFull)
	require.Equal(1, nPartial)
}

func TestGetContractSourceFiles(t *testing.T) {
	require := require.New(t)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v2/contract/42261/0x8259824f2379ef1473a022cBE074076dF8B88444") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockGetContractSourceFilesResponse)
	}))
	defer testServer.Close()

	testClient, err := sourcify.NewClient(testServer.URL, common.ChainNameTestnet, log.NewDefaultLogger("testing/sourcify"))
	require.NoError(err, "NewClient")

	addr := ethCommon.HexToAddress("0x8259824f2379ef1473a022cBE074076dF8B88444")
	sourceFiles, metadata, err := testClient.GetContractSourceFiles(context.Background(), common.RuntimeEmerald, addr)
	require.NoError(err, "GetContractSourceFiles")

	var metadataMap map[string]interface{}
	require.NoError(json.Unmarshal(metadata, &metadataMap), "metadata response unmarshal")

	require.Len(sourceFiles, 12, "GetContractSourceFiles")
	require.Equal(metadataMap["compiler"].(map[string]interface{})["version"], "0.8.24+commit.e11b9ed9", "Metadata.Compiler.Version")

	// Test non-existing contract.
	_, _, err = testClient.GetContractSourceFiles(context.Background(), common.RuntimeCipher, addr)
	require.Error(err, "GetContractSourceFiles")
}
