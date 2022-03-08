// Package processor implements the processor sub-command.
package processor

import (
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
	"github.com/spf13/cobra"
)

const (
	// CfgAddressConsensus is the gRPC target of the node from which
	// consensus blocks will be retrieved.
	CfgAddressConsensus = "address.consensus"
)

var (
	cfgAddressConsensus string

	processCmd = &cobra.Command{
		Use:   "process",
		Short: "Process blocks",
		Run:   runProcessor,
	}
)

func runProcessor(cmd *cobra.Command, args []string) {
	common.Init()
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	processCmd.Flags().StringVar(&cfgAddressConsensus, CfgAddressConsensus, "unix:internal.sock", "consensus gRPC address")

	parentCmd.AddCommand(processCmd)
}
