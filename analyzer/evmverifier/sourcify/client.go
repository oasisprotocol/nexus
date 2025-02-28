package sourcify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
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
// API docs: https://docs.sourcify.dev/docs/api/#/Contract%20Lookup/get-v2-contracts-chainId
//
// Note: This uses the free, public server API. If it turns out to be unreliable, we could use the repository API (vis IPFS proxy) instead, e.g.:
// http://ipfs.default:8080/ipns/repo.sourcify.dev/contracts/full_match/23294
func (s *SourcifyClient) GetVerifiedContractAddresses(ctx context.Context, runtime common.Runtime) (map[ethCommon.Address]VerificationLevel, error) {
	u := *s.serverUrl
	u.Path = path.Join(u.Path, "v2/contracts/", sourcifyChains[s.chain][runtime])

	addresses := make(map[ethCommon.Address]VerificationLevel)
	var lastMatchId string
	for {
		q := u.Query()
		q.Set("limit", "200")
		q.Set("sort", "desc")
		if lastMatchId != "" {
			q.Set("afterMatchId", lastMatchId)
		}
		u.RawQuery = q.Encode()

		body, err := s.callAPI(ctx, http.MethodGet, &u)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch contract addresses from %s: %w", u.String(), err)
		}
		// Parse response.
		var response struct {
			Results []struct {
				Address ethCommon.Address `json:"address"`
				Match   string            `json:"match"`
				MatchId string            `json:"matchId"`
			} `json:"results"`
		}

		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse contract addresses: %w (%s)", err, u.String())
		}
		if len(response.Results) == 0 {
			break
		}
		for _, result := range response.Results {
			switch result.Match {
			case "exact_match":
				addresses[result.Address] = VerificationLevelFull
			case "match":
				addresses[result.Address] = VerificationLevelPartial
			default:
				s.logger.Warn("unknown contract match type", "match", result.Match, "address", result.Address)
			}
		}

		lastMatchId = response.Results[len(response.Results)-1].MatchId
		time.Sleep(100 * time.Millisecond)
	}

	return addresses, nil
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
// The second return argument is the metadata.json file content as a raw JSON message.
//
// See https://docs.sourcify.dev/docs/api/#/Contract%20Lookup/get-contract for more details.
//
// Note: This uses the free, public server API. If it turns out to be unreliable, we could use the repository API (vis IPFS proxy) instead, e.g.:
// - https://docs.sourcify.dev/docs/api/repository/get-file-repository/
// - http://ipfs.default:8080/ipns/repo.sourcify.dev/contracts/full_match/23294/0x0a0b58b5e6d8f2c0f4c4b6e7a0c8f0b1b4b3b2b1/
func (s *SourcifyClient) GetContractSourceFiles(ctx context.Context, runtime common.Runtime, address ethCommon.Address) ([]SourceFile, json.RawMessage, error) {
	// Fetch contract source files.
	u := *s.serverUrl
	u.Path = path.Join(u.Path, "v2/contract/", sourcifyChains[s.chain][runtime], "/", address.String())
	q := u.Query()
	q.Set("fields", "sources,metadata")
	u.RawQuery = q.Encode()
	body, err := s.callAPI(ctx, http.MethodGet, &u)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch contract source files: %w (%s)", err, u.String())
	}

	// Ensure response is a valid JSON.
	var response struct {
		Sources  map[string]map[string]string `json:"sources"`
		Metadata json.RawMessage              `json:"metadata"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, nil, fmt.Errorf("failed to parse contract data: %w (%s)", err, u.String())
	}
	files := make([]SourceFile, 0, len(response.Sources))
	for path, content := range response.Sources {
		// Extract file name from path.
		name := path
		parts := strings.Split(name, "/")
		if len(parts) > 0 {
			name = parts[len(parts)-1]
		}
		files = append(files, SourceFile{
			Name:    name,
			Path:    path,
			Content: content["content"],
		})
	}
	// Sort files by path to have a consistent order.
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return files, response.Metadata, nil
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
