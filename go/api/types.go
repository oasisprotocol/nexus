package api

// APIMetadata is the Oasis Indexer API metadata.
type APIMetadata struct {
	Major uint16
	Minor uint16
	Patch uint16
}

// APIStatus is the Oasis Indexer indexing status.
type APIStatus struct {
	// CurrentHeight is the maximum height the indexer has finished indexing,
	// and is ready for querying.
	CurrentHeight int64
}
