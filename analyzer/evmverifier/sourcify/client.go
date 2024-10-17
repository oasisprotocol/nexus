package sourcify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
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
		common.RuntimeEmerald:     "42261",
		common.RuntimeSapphire:    "23295",
		common.RuntimePontusxTest: "23295", // XXX: We're stealing sapphire data here. TODO: use dedicated verifications.
		common.RuntimePontusxDev:  "23295", // XXX: We're stealing sapphire data here. TODO: use dedicated verifications.
	},
	common.ChainNameMainnet: {
		common.RuntimeEmerald:     "42262",
		common.RuntimeSapphire:    "23294",
		common.RuntimePontusxTest: "23294", // XXX: We're stealing sapphire data here. TODO: use dedicated verifications.
		common.RuntimePontusxDev:  "23294", // XXX: We're stealing sapphire data here. TODO: use dedicated verifications.
	},
}

// Level of contract verification, as defined by Sourcify.
// Keep values in sync with the `sourcify_level` postgres ENUM.
type VerificationLevel string

const (
	VerificationLevelPartial VerificationLevel = "partial"
	VerificationLevelFull    VerificationLevel = "full"
)

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
// API docs: https://sourcify.dev/server/api-docs/#/Repository/get_files_contracts__chain_
//
// Note: This uses the free, public server API. If it turns out to be unreliable, we could use the repository API (vis IPFS proxy) instead, e.g.:
// http://ipfs.default:8080/ipns/repo.sourcify.dev/contracts/full_match/23294
func (s *SourcifyClient) GetVerifiedContractAddresses(ctx context.Context, runtime common.Runtime) (map[ethCommon.Address]VerificationLevel, error) {
	// Build map of addresses.
	addresses := make(map[ethCommon.Address]VerificationLevel)

	// Fetch fully verified contract addresses.
	// See https://docs.sourcify.dev/docs/full-vs-partial-match/
	u := *s.serverUrl
	u.Path = path.Join(u.Path, "files/contracts/full", sourcifyChains[s.chain][runtime])
	fullyVerified, err := s.GetAllAddressPages(ctx, &u)
	if err != nil {
		return nil, err
	}
	for _, addr := range fullyVerified {
		addresses[addr] = VerificationLevelFull
	}
	// Fetch partially verified contract addresses.
	u = *s.serverUrl
	u.Path = path.Join(u.Path, "files/contracts/any", sourcifyChains[s.chain][runtime])
	allContracts, err := s.GetAllAddressPages(ctx, &u)
	if err != nil {
		return nil, err
	}
	for _, addr := range allContracts {
		if _, exists := addresses[addr]; !exists {
			addresses[addr] = VerificationLevelPartial
		}
	}

	return addresses, nil
}

func (s *SourcifyClient) GetAllAddressPages(ctx context.Context, u *url.URL) ([]ethCommon.Address, error) {
	page := 0
	addresses := []ethCommon.Address{}
	for {
		// https://sourcify.dev/server/api-docs/#/Repository/get_files_contracts_any__chain_
		q := u.Query()
		q.Set("page", fmt.Sprintf("%d", page))
		u.RawQuery = q.Encode()

		body, err := s.callAPI(ctx, http.MethodGet, u)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch fully verified contract addresses from %s: %w", u.String(), err)
		}
		// Parse response.
		var response struct {
			Results    []ethCommon.Address `json:"results"`
			Pagination SourcifyPagination  `json:"pagination"`
		}

		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse fully verified contract addresses: %w (%s)", err, u.String())
		}
		addresses = append(addresses, response.Results...)
		// Check pagination and increment if necessary.
		if !response.Pagination.HasNextPage {
			break
		}
		page++
	}

	return addresses, nil
}

type SourcifyPagination struct {
	CurrentPage        uint64 `json:"currentPage"`
	TotalPages         uint64 `json:"totalPages"`
	ResultsPerPage     uint64 `json:"resultsPerPage"`
	ResultsCurrentPage uint64 `json:"resultsCurrentPage"`
	TotalResults       uint64 `json:"totalResults"`
	HasNextPage        bool   `json:"hasNextPage"`
	HasPreviousPage    bool   `json:"hasPreviousPage"`
}

type SourceFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

// GetContractSourceFiles returns the source files for the given contract address that is verified on sourcify.
// The first return argument are the source files returned in a list of maps, where each map contains the following keys:
// - "name": The name of the source file.
// - "content": The content of the source file.
// - "path": The path of the source file.
// For convenience the second return argument is the metadata.json file content as a raw JSON message.
//
// See https://sourcify.dev/server/api-docs/#/Repository/get_files_any__chain___address_ for more details.
//
// Note: This uses the free, public server API. If it turns out to be unreliable, we could use the repository API (vis IPFS proxy) instead, e.g.:
// - https://docs.sourcify.dev/docs/api/repository/get-file-repository/
// - http://ipfs.default:8080/ipns/repo.sourcify.dev/contracts/full_match/23294/0x0a0b58b5e6d8f2c0f4c4b6e7a0c8f0b1b4b3b2b1/
func (s *SourcifyClient) GetContractSourceFiles(ctx context.Context, runtime common.Runtime, address ethCommon.Address) ([]SourceFile, json.RawMessage, error) {
	// Fetch contract source files.
	u := *s.serverUrl
	u.Path = path.Join(u.Path, "files/any", sourcifyChains[s.chain][runtime], address.String())
	body, err := s.callAPI(ctx, http.MethodGet, &u)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch contract source files: %w (%s)", err, u.String())
	}

	// Ensure response is a valid JSON.
	var response struct {
		Files []SourceFile `json:"files"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, nil, fmt.Errorf("failed to parse contract source files: %w (%s)", err, u.String())
	}

	// Find metadata.json.
	var metadata json.RawMessage
	found := -1
	for i, file := range response.Files {
		if file.Name == "metadata.json" {
			if err = json.Unmarshal([]byte(file.Content), &metadata); err != nil {
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
		response.Files = append(response.Files[:found], response.Files[found+1:]...)
	}

	return response.Files, metadata, nil
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
