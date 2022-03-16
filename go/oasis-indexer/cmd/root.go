// Package cmd implements commands for the processor executable.
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/oasislabs/oasis-block-indexer/go/metrics"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/analyzer"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/api"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
)

var rootCmd = &cobra.Command{
	Use:   "oasis-indexer",
	Short: "Oasis Indexer",
	Run:   rootMain,
}

func rootMain(cmd *cobra.Command, args []string) {
	if err := common.Init(); err != nil {
		os.Exit(1)
	}
	rootLogger := common.Logger()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	_, cancelFn := context.WithCancel(context.Background())
	go func() {
		<-sigCh
		rootLogger.Info("user requested interrupt")
		cancelFn()
	}()

	// Initialize Prometheus service
	promServer, err := metrics.NewPullService(rootLogger)
	if err != nil {
		rootLogger.Error("failed to initialize metrics", "err", err)
		os.Exit(1)
	}
	promServer.StartInstrumentation()

	// TODO: Start oasis-indexer

	rootLogger.Info("terminating")
}

// Execute spawns the main entry point after handing the config file.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	common.RegisterFlags(rootCmd)

	for _, f := range []func(*cobra.Command){
		analyzer.Register,
		api.Register,
	} {
		f(rootCmd)
	}
}
