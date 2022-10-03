package tests

const (
	GenesisHeight = int64(1)  // TODO: Get from config.
	ChainID       = "oasis-3" // TODO: Get from config.
)

var baseEndpoint string

// Init initializes the testing environment.
func Init() {
	baseEndpoint = "http://localhost:8008/v1"
}
