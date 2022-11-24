package tests

const (
	GenesisHeight = int64(8_048_956) // TODO: Get from config.
	ChainID       = "oasis-test"     // TODO: Get from config.
)

var baseEndpoint string

// Init initializes the testing environment.
func Init() {
	baseEndpoint = "http://localhost:8008/v1"
}
