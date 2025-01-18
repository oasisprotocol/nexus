package metadata_registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"

	registry "github.com/oasisprotocol/metadata-registry-tools"

	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/httpmisc"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/pubclient"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
)

const (
	keybaseMaxResponseSize = 10 * 1024 * 1024 // 10 MiB.
	keybaseLookupUrl       = "https://keybase.io/_/api/1.0/user/lookup.json?fields=pictures&usernames="
)

const MetadataRegistryAnalyzerName = "metadata_registry"

type processor struct {
	target storage.TargetStorage
	logger *log.Logger

	gitCfg       registry.GitConfig
	mockLogoUrls bool
}

var _ item.ItemProcessor[struct{}] = (*processor)(nil)

func NewAnalyzer(
	cfg config.MetadataRegistryConfig,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	logger.Info("Starting metadata_registry analyzer")
	if cfg.Interval == 0 {
		cfg.Interval = 2 * time.Minute
	}
	logger = logger.With("analyzer", MetadataRegistryAnalyzerName)
	p := &processor{
		target:       target,
		logger:       logger,
		gitCfg:       registry.NewGitConfig(),
		mockLogoUrls: cfg.MockLogoUrls,
	}
	if cfg.RepositoryBranch != "" {
		p.gitCfg.Branch = cfg.RepositoryBranch
	}

	return item.NewAnalyzer[struct{}](
		MetadataRegistryAnalyzerName,
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
	gp, err := registry.NewGitProvider(p.gitCfg)
	if err != nil {
		return fmt.Errorf("failed to create Git registry provider: %s", err)
	}

	// Get a list of all entities in the registry.
	entities, err := gp.GetEntities(ctx)
	if err != nil {
		return fmt.Errorf("failed to get a list of entities in registry: %s", err)
	}

	for id, meta := range entities {
		var logoUrl string
		if meta.Keybase != "" {
			logoUrl, err = fetchKeybaseLogoUrl(ctx, meta.Keybase)
			if err != nil {
				p.logger.Warn("failed to fetch keybase url", "err", err, "handle", meta.Keybase)
			}
			if ctx.Err() != nil {
				// Exit early if the context is done.
				return ctx.Err()
			}
		}
		if p.mockLogoUrls {
			logoUrl = "http:://e2e-tests-mock-static-logo-url"
		}

		batch.Queue(
			queries.ConsensusEntityMetaUpsert,
			id.String(),
			staking.NewAddress(id).String(),
			meta,
			logoUrl,
		)
	}

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	// The concept of a work queue does not apply to this analyzer.
	return 0, nil
}

func fetchKeybaseLogoUrl(ctx context.Context, handle string) (string, error) {
	resp, err := pubclient.GetWithContext(ctx, keybaseLookupUrl+url.QueryEscape(handle))
	if err != nil {
		return "", err
	}
	if err = httpmisc.ResponseOK(resp); err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return parseKeybaseLogoUrl(io.LimitReader(resp.Body, keybaseMaxResponseSize))
}

func parseKeybaseLogoUrl(r io.Reader) (string, error) {
	var response struct {
		Them []struct {
			Id       string `json:"id"`
			Pictures *struct {
				Primary struct {
					Url string `json:"url"`
				} `json:"primary"`
			} `json:"pictures"`
		} `json:"them"`
	}
	if err := json.NewDecoder(r).Decode(&response); err != nil {
		return "", err
	}

	if len(response.Them) > 0 && response.Them[0].Pictures != nil {
		return response.Them[0].Pictures.Primary.Url, nil
	}
	return "", nil
}
