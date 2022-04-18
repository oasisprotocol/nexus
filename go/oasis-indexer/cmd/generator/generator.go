// Package generator implements the `generate` sub-command. This is intended
// to primarily be a utility command for generating migrations for populating
// the Oasis Indexer database from genesis state.
package generator

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-block-indexer/go/storage/migrations/generator"
	"github.com/oasislabs/oasis-block-indexer/go/storage/oasis"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

const (
	// CfgOutputFilename is the file to which generated migrations should
	// be written.
	CfgOutputFilename = "generator.output_filename"

	// CfgChainID is the target chain ID for which the migration should
	// be generated.
	CfgChainID = "generator.chain_id"

	// CfgNetworkConfig is the config file for connecting to an oasis-node.
	CfgNetworkConfig = "generator.network_config"

	moduleName = "generator"
)

var (
	cfgOutputFilename string
	cfgChainID        string
	cfgNetworkConfig  string

	generateCmd = &cobra.Command{
		Use:   "generate",
		Short: "Generate migrations",
		Run:   runGenerator,
	}
)

func runGenerator(cmd *cobra.Command, args []string) {
	if err := common.Init(); err != nil {
		os.Exit(1)
	}

	logger := common.Logger().WithModule(moduleName)
	g := generator.NewMigrationGenerator(logger)

	w := os.Stdout
	if cfgOutputFilename != "" {
		f, err := os.Create(cfgOutputFilename)
		if err != nil {
			logger.Error("failed to create migrations file",
				"error", err,
			)
			os.Exit(1)
		}
		defer f.Close()

		w = f
	}

	rawCfg, err := ioutil.ReadFile(cfgNetworkConfig)
	if err != nil {
		logger.Error("failed to parse network config",
			"error", err,
		)
		os.Exit(1)
	}

	var network config.Network
	yaml.Unmarshal([]byte(rawCfg), &network)

	ctx := context.Background()
	client, err := oasis.NewOasisNodeClient(ctx, &network)
	if err != nil {
		logger.Error("failed to create oasis-node client",
			"error", err,
		)
		os.Exit(1)
	}

	d, err := client.GenesisDocument(ctx)
	if err != nil {
		logger.Error("failed to fetch genesis document",
			"error", err,
		)
		os.Exit(1)
	}

	if cfgChainID == "" {
		cfgChainID = d.ChainID
	}

	switch cfgChainID {
	case "oasis-3":
		if err := g.WriteGenesisDocumentMigrationOasis3(w, d); err != nil {
			logger.Error("failed to write migration",
				"error", err,
			)
		}
		break
	default:
		logger.Error("unsupported chain id")
		os.Exit(1)
	}

	logger.Info("successfully wrote migration",
		"output_file", cfgOutputFilename,
	)
	os.Exit(0)
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	generateCmd.Flags().StringVar(&cfgOutputFilename, CfgOutputFilename, "", "path to output migration file")
	generateCmd.Flags().StringVar(&cfgChainID, CfgChainID, "", "chain id to target for migration generation")
	generateCmd.Flags().StringVar(&cfgNetworkConfig, CfgNetworkConfig, "", "path to a network configuration file")
	parentCmd.AddCommand(generateCmd)
}
