// Package analyzer implements the analyzer sub-command.
package analyzer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/spf13/cobra"

	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
	"github.com/oasislabs/oasis-block-indexer/go/storage/oasis"
)

const (
	// CfgStorageEndpoint is the flag for setting the connection string to
	// the backing storage.
	CfgStorageEndpoint = "storage.endpoint"

	moduleName = "analysis"
)

var (
	cfgStorageEndpoint string

	analyzeCmd = &cobra.Command{
		Use:   "analyze",
		Short: "Analyze blocks",
		Args:  cobra.ExactArgs(6),
		Run:   runAnalyzer,
	}
)

func runAnalyzer(cmd *cobra.Command, args []string) {
	_, chainContext, rpc, description, symbol, decimals := args[0], args[1], args[2], args[3], args[4], args[5]

	network := config.Network{
		ChainContext: chainContext,
		RPC:          rpc,
	}

	decimalsInt, err := strconv.ParseUint(decimals, 10, 8)
	cobra.CheckErr(err)

	network.Description = description
	network.Denomination.Symbol = symbol
	network.Denomination.Decimals = uint8(decimalsInt)

	common.Init()

	analyzer, err := NewAnalyzer(network)
	cobra.CheckErr(err)

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
	document, err := client.GenesisDocument(c)
	cobra.CheckErr(err)
	initialHeight := document.Height
	height := initialHeight

	for height < (initialHeight + 10) {
		fmt.Println(height)
		blockData, err := client.BlockData(c, height)
		cobra.CheckErr(err)
		fmt.Println(blockData)
		registryData, err := client.RegistryData(c, height)
		cobra.CheckErr(err)
		fmt.Println(registryData)
		stakingData, err := client.StakingData(c, height)
		cobra.CheckErr(err)
		fmt.Println(stakingData)
		schedulerData, err := client.SchedulerData(c, height)
		cobra.CheckErr(err)
		fmt.Println(schedulerData)
		governanceData, err := client.GovernanceData(c, height)
		cobra.CheckErr(err)
		fmt.Println(governanceData)
		height += 1
	}
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	analyzeCmd.Flags().StringVar(&cfgStorageEndpoint, CfgStorageEndpoint, "", "a postgresql-compliant connection url")

	parentCmd.AddCommand(analyzeCmd)
}
