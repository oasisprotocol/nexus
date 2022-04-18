// Package analyzer implements the `analyze` sub-command.
package analyzer

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
	"github.com/oasislabs/oasis-block-indexer/go/storage/oasis"
)

const (
	// CfgStorageEndpoint is the flag for setting the connection string to
	// the backing storage.
	CfgStorageEndpoint = "analyzer.storage_endpoint"

	// CfgNetworkConfig is the config file for connecting to an oasis-node.
	CfgNetworkConfig = "analyzer.network_config"

	moduleName = "analysis_service"
)

var (
	cfgStorageEndpoint string
	cfgNetworkConfig   string

	analyzeCmd = &cobra.Command{
		Use:   "analyze",
		Short: "Analyze blocks",
		Run:   runAnalyzer,
	}
)

func runAnalyzer(cmd *cobra.Command, args []string) {
	if err := common.Init(); err != nil {
		os.Exit(1)
	}

	rawCfg, err := ioutil.ReadFile(cfgNetworkConfig)
	if err != nil {
		common.Logger().Error(
			"failed to parse network config",
			"error", err,
		)
		os.Exit(1)
	}

	var network config.Network
	yaml.Unmarshal([]byte(rawCfg), &network)

	analyzer, err := NewAnalyzer(network)
	if err != nil {
		common.Logger().Error(
			"failed to create analyzer",
			"error", err,
		)
		os.Exit(1)
	}

	switch {
	case err == nil:
		analyzer.Start()
	case errors.Is(err, context.Canceled):
		// Shutdown requested during startup.
		return
	default:
		os.Exit(1)
	}
}

// Analyzer is the Oasis Indexer's analysis service.
type Analyzer struct {
	Network       config.Network
	TargetStorage storage.TargetStorage
	logger        *log.Logger
}

// NewAnalyzer creates and starts a new Analyzer
func NewAnalyzer(net config.Network) (*Analyzer, error) {
	logger := common.Logger().WithModule(moduleName)

	analyzer := &Analyzer{
		logger:  logger,
		Network: net,
	}

	logger.Info("Starting oasis-indexer analysis layer.")

	return analyzer, nil
}

func (analyzer *Analyzer) Start() {
	c := context.Background()
	client, err := oasis.NewOasisNodeClient(c, &analyzer.Network)

	if err != nil {
		common.Logger().Error("Could not create Oasis node client")
		os.Exit(1)
	}

	document, err := client.GenesisDocument(c)

	if err != nil {
		common.Logger().Error("Could not retrieve genesis document from client")
		os.Exit(1)
	}

	initialHeight := document.Height
	height := initialHeight + 1

	for height < (initialHeight + 10) {
		fmt.Println(height)
		blockData, err := client.BlockData(c, height)
		if err != nil {
			os.Exit(1)
		}

		fmt.Println(blockData)
		registryData, err := client.RegistryData(c, height)
		if err != nil {
			os.Exit(1)
		}

		fmt.Println(registryData)
		stakingData, err := client.StakingData(c, height)
		if err != nil {
			os.Exit(1)
		}

		fmt.Println(stakingData)
		schedulerData, err := client.SchedulerData(c, height)
		if err != nil {
			os.Exit(1)
		}

		fmt.Println(schedulerData)
		governanceData, err := client.GovernanceData(c, height)
		if err != nil {
			os.Exit(1)
		}

		fmt.Println(governanceData)
		height += 1
	}
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	analyzeCmd.Flags().StringVar(&cfgStorageEndpoint, CfgStorageEndpoint, "", "a postgresql-compliant connection url")
	analyzeCmd.Flags().StringVar(&cfgNetworkConfig, CfgNetworkConfig, "", "path to a network configuration file")
	parentCmd.AddCommand(analyzeCmd)
}
