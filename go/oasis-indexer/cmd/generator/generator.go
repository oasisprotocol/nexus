// Package generator implements the `generate` sub-command. This is intended
// to primarily be a utility command for generating migrations for populating
// the Oasis Indexer database from genesis state.
package generator

import (
	"context"
	"errors"
	"io/ioutil"
	"os"

	"github.com/oasislabs/oasis-block-indexer/go/log"
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

	g, err := NewGenerator()
	switch {
	case err == nil:
		if err := g.WriteMigration(); err != nil {
			common.Logger().Error("migration failed to run",
				"error", err,
			)
			os.Exit(1)
		}
		return
	case errors.Is(err, context.Canceled):
		// Shutdown requested during startup.
		return
	default:
		common.Logger().Error("generator failed to initialize",
			"error", err,
		)
		os.Exit(1)
	}
}

// Generator is the Oasis Indexer's migration generator.
type Generator struct {
	gen    *generator.MigrationGenerator
	client *oasis.OasisNodeClient
	logger *log.Logger
}

// NewGenerator creates a new Generator.
func NewGenerator() (*Generator, error) {
	logger := common.Logger().WithModule(moduleName)

	// Connect to oasis-node.
	rawCfg, err := ioutil.ReadFile(cfgNetworkConfig)
	if err != nil {
		return nil, err
	}

	var network config.Network
	yaml.Unmarshal([]byte(rawCfg), &network)

	ctx := context.Background()
	client, err := oasis.NewOasisNodeClient(ctx, &network)
	if err != nil {
		return nil, err
	}

	return &Generator{
		gen:    generator.NewMigrationGenerator(logger),
		client: client,
		logger: logger,
	}, nil
}

// WriteMigration writes the state migration.
func (g *Generator) WriteMigration() error {
	ctx := context.Background()

	// Fetch genesis document for migration.
	d, err := g.client.GenesisDocument(ctx)
	if err != nil {
		return err
	}

	// Create output file.
	w := os.Stdout
	if cfgOutputFilename != "" {
		f, err := os.Create(cfgOutputFilename)
		if err != nil {
			return err
		}
		defer f.Close()

		w = f
	}

	// Generate migration.
	switch d.ChainID {
	case "oasis-3":
		if err := g.gen.WriteGenesisDocumentMigrationOasis3(w, d); err != nil {
			return err
		}
		break
	default:
		g.logger.Error("unsupported chain id")
		return errors.New("unsupported chain id")
	}

	g.logger.Info("successfully wrote migration")
	return nil
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	generateCmd.Flags().StringVar(&cfgOutputFilename, CfgOutputFilename, "", "path to output migration file")
	generateCmd.Flags().StringVar(&cfgChainID, CfgChainID, "", "chain id to target for migration generation")
	generateCmd.Flags().StringVar(&cfgNetworkConfig, CfgNetworkConfig, "", "path to a network configuration file")
	parentCmd.AddCommand(generateCmd)
}
