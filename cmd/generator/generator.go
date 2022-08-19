// Package generator implements the `generate` sub-command. This is intended
// to primarily be a utility command for generating migrations for populating
// the Oasis Indexer database from genesis state.
package generator

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	genesis "github.com/oasisprotocol/oasis-core/go/genesis/api"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/oasis-indexer/cmd/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage/generator"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis"
)

const (
	// CfgMigrationFile is the file to which generated migrations should
	// be written.
	CfgMigrationFile = "generator.migration_file"

	// CfgGenesisFile is the file from which the genesis document used
	// to generate migrations should be loaded.
	CfgGenesisFile = "generator.genesis_file"

	// CfgNetworkConfigFile is the config file for connecting to an oasis-node.
	CfgNetworkConfigFile = "generator.network_config_file"

	moduleName = "generator"
)

var (
	// Path to the configuration file.
	configFile string

	cfgMigrationFile     string
	cfgGenesisFile       string
	cfgNetworkConfigFile string

	generateCmd = &cobra.Command{
		Use:   "generate",
		Short: "Generate migrations",
		Run:   runGenerator,
	}
)

func runGenerator(cmd *cobra.Command, args []string) {
	// Initialize config.
	cfg, err := config.InitConfig(configFile)
	if err != nil {
		os.Exit(1)
	}

	// Initialize common environment.
	if err = common.Init(cfg); err != nil {
		os.Exit(1)
	}
	logger := common.Logger()

	g, err := NewGenerator()
	if err != nil {
		logger.Error("migration failed to run",
			"error", err,
		)
		os.Exit(1)
	}
	if err := g.WriteMigration(); err != nil {
		logger.Error("generator failed to initialize",
			"error", err,
		)
		os.Exit(1)
	}
}

// Generator is the Oasis Indexer's migration generator.
type Generator struct {
	gen    *generator.MigrationGenerator
	logger *log.Logger
}

// NewGenerator creates a new Generator.
func NewGenerator() (*Generator, error) {
	logger := common.Logger().WithModule(moduleName)

	return &Generator{
		gen:    generator.NewMigrationGenerator(logger),
		logger: logger,
	}, nil
}

// WriteMigration writes the state migration.
func (g *Generator) WriteMigration() error {
	var d *genesis.Document
	switch {
	case cfgGenesisFile != "":
		doc, err := g.genesisDocFromFile()
		if err != nil {
			return err
		}
		d = doc
	case cfgNetworkConfigFile != "":
		doc, err := g.genesisDocFromClient()
		if err != nil {
			return err
		}
		d = doc
	default:
		return errors.New("neither genesis file nor network config provided")
	}

	// Create output file.
	w := os.Stdout
	if cfgMigrationFile != "" {
		var err error
		w, err = os.Create(cfgMigrationFile)
		if err != nil {
			return err
		}
		defer w.Close()
	}

	// Generate migration.
	switch d.ChainID {
	case "oasis-3":
		if err := g.gen.WriteGenesisDocumentMigrationOasis3(w, d); err != nil {
			return err
		}
	case "test":
		if err := g.gen.WriteGenesisDocumentMigrationOasis3(w, d); err != nil {
			return err
		}
	default:
		g.logger.Error("unsupported chain id")
		return errors.New("unsupported chain id")
	}

	g.logger.Info("successfully wrote migration")
	return nil
}

func (g *Generator) genesisDocFromFile() (*genesis.Document, error) {
	rawDoc, err := ioutil.ReadFile(cfgGenesisFile)
	if err != nil {
		return nil, err
	}

	var d genesis.Document
	if err := json.Unmarshal(rawDoc, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (g *Generator) genesisDocFromClient() (*genesis.Document, error) {
	ctx := context.Background()

	// Connect to oasis-node.
	rawCfg, err := ioutil.ReadFile(cfgNetworkConfigFile)
	if err != nil {
		return nil, err
	}

	var network oasisConfig.Network
	if err = yaml.Unmarshal(rawCfg, &network); err != nil {
		return nil, err
	}

	client, err := oasis.NewClient(ctx, &network)
	if err != nil {
		return nil, err
	}

	// Fetch genesis document for migration.
	d, err := client.GenesisDocument(ctx)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	generateCmd.Flags().StringVar(&configFile, "config", "./config/local-dev.yml", "path to the config.yml file")
	generateCmd.Flags().StringVar(&cfgMigrationFile, CfgMigrationFile, "", "path to output migration file")
	generateCmd.Flags().StringVar(&cfgGenesisFile, CfgGenesisFile, "", "path to input genesis file")
	generateCmd.Flags().StringVar(&cfgNetworkConfigFile, CfgNetworkConfigFile, "", "path to a network configuration file")
	parentCmd.AddCommand(generateCmd)
}
