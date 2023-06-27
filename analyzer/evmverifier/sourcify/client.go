package sourcify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"

	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
)

const (
	defaultServerUrl = "https://sourcify.dev/server"
)

// Oasis Runtime to Sourcify Chain ID mapping:
// https://docs.sourcify.dev/docs/chains/
var sourcifyChains = map[common.ChainName]map[common.Runtime]string{
	common.ChainNameTestnet: {
		common.RuntimeEmerald:  "42261",
		common.RuntimeSapphire: "23295",
	},
	common.ChainNameMainnet: {
		common.RuntimeEmerald:  "42262",
		common.RuntimeSapphire: "23294",
	},
}

// SourcifyClient is a client for interacting with the Sourcify Server API,
// providing methods to fetch and parse contract data for supported Oasis runtimes.
type SourcifyClient struct {
	serverUrl  *url.URL
	chain      common.ChainName
	httpClient *http.Client
	logger     *log.Logger
}

func (s *SourcifyClient) callAPI(ctx context.Context, method string, url *url.URL) ([]byte, error) {
	s.logger.Debug("sourcify API call", "method", method, "url", url.String())
	req, err := http.NewRequestWithContext(ctx, method, url.String(), nil)
	if err != nil {
		return nil, err
	}
	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sourcify API call failure: %w", err)
	}
	defer res.Body.Close()
	resp, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	s.logger.Debug("sourcify API call response", "method", method, "url", url.String(), "status", res.Status, "response_body", string(resp))
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sourcify API call failure: %s", res.Status)
	}

	return resp, nil
}

// GetVerifiedContractAddresses returns a list of all verified contract addresses for the given runtime.
//
// API docs: https://docs.sourcify.dev/docs/api/server/get-contract-addresses-all/
//
// Note: This uses the free, public server API. If it turns out to be unreliable, we could use the repository API (vis IPFS proxy) instead, e.g.:
// http://ipfs.default:8080/ipns/repo.sourcify.dev/contracts/full_match/23294
func (s *SourcifyClient) GetVerifiedContractAddresses(ctx context.Context, runtime common.Runtime) ([]ethCommon.Address, error) {
	// Fetch verified contract addresses.
	u := *s.serverUrl
	u.Path = path.Join(u.Path, "files/contracts", sourcifyChains[s.chain][runtime])
	body, err := s.callAPI(ctx, http.MethodGet, &u)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch verified contract addresses: %w (%s)", err, u.String())
	}

	// Parse response.
	var response struct {
		Full []ethCommon.Address `json:"full"`
		// XXX: Skip partial matches for now: https://docs.sourcify.dev/docs/full-vs-partial-match/
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse verified contract addresses: %w (%s)", err, u.String())
	}

	return response.Full, nil
}

// GetContractSourceFiles returns the source files for the given contract address that is verified on sourcify.
// The first return argument are the source files returned in a list of maps, where each map contains the following keys:
// - "name": The name of the source file.
// - "content": The content of the source file.
// - "path": The path of the source file.
// For convenience the second return argument is the metadata.json file content as a raw JSON message.
//
// See https://docs.sourcify.dev/docs/api/server/get-source-files-full/ for more details.
//
// Note: This uses the free, public server API. If it turns out to be unreliable, we could use the repository API (vis IPFS proxy) instead, e.g.:
// - https://docs.sourcify.dev/docs/api/repository/get-file-repository/
// - http://ipfs.default:8080/ipns/repo.sourcify.dev/contracts/full_match/23294/0x0a0b58b5e6d8f2c0f4c4b6e7a0c8f0b1b4b3b2b1/
func (s *SourcifyClient) GetContractSourceFiles(ctx context.Context, runtime common.Runtime, address ethCommon.Address) ([]map[string]json.RawMessage, json.RawMessage, error) {
	// Fetch contract source files.
	u := *s.serverUrl
	u.Path = path.Join(u.Path, "files", sourcifyChains[s.chain][runtime], address.String())
	body, err := s.callAPI(ctx, http.MethodGet, &u)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch contract source files: %w (%s)", err, u.String())
	}

	// Ensure response is a valid JSON.
	var response []map[string]json.RawMessage // Response is an array of source files.
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, nil, fmt.Errorf("failed to parse contract source files: %w (%s)", err, u.String())
	}

	// Find metadata.json.
	var metadata json.RawMessage
	found := -1
	for i, file := range response {
		var name string
		if err = json.Unmarshal(file["name"], &name); err != nil {
			continue
		}
		if name == "metadata.json" {
			// Remove the escaped quotes from the metadata json content string.
			content, err := strconv.Unquote(string(file["content"]))
			if err != nil {
				s.logger.Warn("failed to parse contract metadata string", "err", err, "url", u.String())
				continue
			}
			if err = json.Unmarshal([]byte(content), &metadata); err != nil {
				s.logger.Warn("failed to unmarshal contract metadata", "err", err, "url", u.String())
				continue
			}
			found = i
			break
		}
	}
	if found == -1 {
		s.logger.Warn("failed to find metadata.json in source files", "url", u.String())
	} else {
		// Remove metadata.json from the source files list.
		response = append(response[:found], response[found+1:]...)
	}

	return response, metadata, nil
}

// NewClient returns a new Sourcify API client.
func NewClient(serverUrl string, chain common.ChainName, logger *log.Logger) (*SourcifyClient, error) {
	if serverUrl == "" {
		serverUrl = defaultServerUrl
	}
	url, err := url.Parse(serverUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server url: %w", err)
	}

	return &SourcifyClient{
		serverUrl:  url,
		chain:      chain,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}, nil
}
