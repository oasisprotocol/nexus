// Package processor implements the processor sub-command.
package processor

import (
	"github.com/spf13/cobra"
)

const (
	// CfgEndpointConsensus is the gRPC target of the node from which
	// consensus blocks will be retrieved.
	CfgEndpointConsensus = "endpoint.consensus"
)

var (
	endpointConsensus string

	processCmd = &cobra.Command{
		Use:   "process",
		Short: "Process blocks",
		Run:   runProcessor,
	}
)

func runProcessor(cmd *cobra.Command, args []string) {
	// TODO
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	processCmd.Flags().StringVar(&endpointConsensus, CfgEndpointConsensus, "", "consensus node grpc target")

	parentCmd.AddCommand(processCmd)
}
