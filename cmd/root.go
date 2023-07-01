// Package cmd implements commands for the processor executable.
package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/nexus/cmd/analyzer"
	"github.com/oasisprotocol/nexus/cmd/api"
	"github.com/oasisprotocol/nexus/cmd/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
)

var (
	// Path to the configuration file.
	configFile string

	rootCmd = &cobra.Command{
		Use:   "nexus",
		Short: "Oasis Nexus",
		Run:   rootMain,
	}
)

// Service is a service run by Nexus.
type Service interface {
	// Start starts the service.
	Start()
}

func rootMain(cmd *cobra.Command, args []string) {
	// Initialize config.
	cfg, err := config.InitConfig(configFile)
	if err != nil {
		log.NewDefaultLogger("init").Error("init failed",
			"error", err,
		)
		os.Exit(1)
	}

	// Initialize common environment.
	if err = common.Init(cfg); err != nil {
		log.NewDefaultLogger("init").Error("init failed",
			"error", err,
		)
		os.Exit(1)
	}
	logger := common.Logger()

	// Initialize services.
	var wg sync.WaitGroup
	runInWG := func(s Service) {
		wg.Add(1)
		go func(s Service) {
			defer wg.Done()
			s.Start()
		}(s)
	}

	if cfg.Analysis != nil {
		analysisService, err := analyzer.Init(cfg.Analysis)
		if err != nil {
			logger.Error("failed to initialize analysis service", "err", err)
			os.Exit(1)
		}
		runInWG(analysisService)
	}
	if cfg.Server != nil {
		apiService, err := api.Init(cfg.Server)
		if err != nil {
			logger.Error("failed to initialize api service", "err", err)
			os.Exit(1)
		}
		runInWG(apiService)
	}

	logger.Info("started all services")
	wg.Wait()
}

// Execute spawns the main entry point after handing the config file.
func Execute() {
	// Debug hook. If we receive SIGUSR1, dump all goroutines.
	go dumpGoroutinesOnSignal(syscall.SIGUSR1)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&configFile, "config", "./conf/server.yml", "path to the config.yml file")

	for _, f := range []func(*cobra.Command){
		analyzer.Register,
		api.Register,
	} {
		f(rootCmd)
	}
}

// Starts listening for the specified signals, and logs a dump of all
// goroutines when the process receives one of those signals.
func dumpGoroutinesOnSignal(signals ...os.Signal) {
	logger := log.NewDefaultLogger("toplevel")
	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)
	logger.Info("listening for signals", "signals", signals)
	for range c {
		b := bytes.NewBufferString("")
		_ = pprof.Lookup("goroutine").WriteTo(b, 1)
		logger.Warn("USER-REQUESTED DUMP: all goroutines", "goroutines_all", b.String())

		b = bytes.NewBufferString("")
		_ = pprof.Lookup("block").WriteTo(b, 1)
		logger.Warn("USER-REQUESTED DUMP: stack traces that led to blocking on synchronization primitives", "goroutines_block", b.String())

		b = bytes.NewBufferString("")
		_ = pprof.Lookup("mutex").WriteTo(b, 1)
		logger.Warn("USER-REQUESTED DUMP: stack traces of holders of contended mutexes", "goroutines_mutex", b.String())
	}
}
