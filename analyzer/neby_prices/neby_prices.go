// Package nebyprices implements the Neby prices analyzer.
package nebyprices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/httpmisc"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/pubclient"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
)

const (
	analyzerPrefix = "neby_prices_"

	defaultNebyEndpoint = "https://graph.api.neby.exchange/dex"
)

type processor struct {
	runtime common.Runtime
	target  storage.TargetStorage
	logger  *log.Logger

	graphEndpoint string
}

var _ item.ItemProcessor[struct{}] = (*processor)(nil)

func NewAnalyzer(
	runtime common.Runtime,
	cfg *config.NebyPricesConfig,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	logger = logger.With("analyzer", analyzerPrefix+runtime)

	logger.Info("Starting analyzer")

	endpoint := cfg.GraphEndpoint
	if endpoint == "" {
		endpoint = defaultNebyEndpoint
	}

	if cfg.Interval == 0 {
		cfg.Interval = 15 * time.Minute
	}
	p := &processor{
		runtime:       runtime,
		target:        target,
		logger:        logger,
		graphEndpoint: endpoint,
	}

	return item.NewAnalyzer(
		analyzerPrefix+string(runtime),
		cfg.ItemBasedAnalyzerConfig,
		p,
		target,
		logger,
	)
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]struct{}, error) {
	return []struct{}{{}}, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item struct{}) error {
	// Fetch the Neby token prices.
	p.logger.Debug("fetching Neby token prices", "endpoint", p.graphEndpoint)
	result, err := fetchNebyTokenPrices(ctx, p.graphEndpoint)
	if err != nil {
		return fmt.Errorf("failed to fetch Neby token prices: %w", err)
	}

	p.logger.Debug("fetched Neby token prices", "count", len(result.Data.Tokens))

	// Store the prices in the database.
	for _, token := range result.Data.Tokens {
		p.logger.Debug("storing Neby token price",
			"token_id", token.ID,
			"derived_eth", token.DerivedETH,
			"name", token.Name,
		)

		ethAddress := ethCommon.HexToAddress(token.ID)
		batch.Queue(
			tokenNebyDerivedPriceUpsert,
			p.runtime,
			ethAddress[:],
			token.DerivedETH,
		)
	}

	return nil
}

type graphTokensResponse struct {
	Data struct {
		Tokens []struct {
			ID         string `json:"id"`
			DerivedETH string `json:"derivedETH"`
			Name       string `json:"name"`
		} `json:"tokens"`
	} `json:"data"`
}

func fetchNebyTokenPrices(ctx context.Context, endpoint string) (*graphTokensResponse, error) {
	query := `
	{
		tokens(first: 1000) {
			id
			derivedETH
			name
		}
	}`
	body, _ := json.Marshal(map[string]string{
		"query": query,
	})

	resp, err := pubclient.PostWithContext(ctx, endpoint, "application/json", body)
	if err != nil {
		return nil, err
	}
	if httpmisc.ResponseOK(resp) != nil {
		return nil, fmt.Errorf("response: %s", resp.Status)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result graphTokensResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	return 0, nil
}
