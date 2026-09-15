package fts

import (
	"github.com/meilisearch/meilisearch-go"
)

// Indexer is the interface that wraps the basic IndexDocuments method.
//
// IndexDocuments indexes the given documents. Though the interface accepts any type, in general this will be
// a slice of map[string]any. Implementations should handle the conversion to the appropriate format.
type Indexer interface {
	IndexDocuments(docs any) error
}

// Client is the interface to the FTS client. Currently, it supports a single call to get a particular index (Indexer).
type Client interface {
	GetIndex(name string) (Indexer, error)
}

// MeilisearchClient is the implementation of the Client interface for Meilisearch.
type MeilisearchClient struct {
	client meilisearch.ServiceManager
}

// NewMeilisearchClient creates a new MeilisearchClient instance. If APIKey is not blank, then it instantiates the
// Meilisearch client with the given key.
func NewMeilisearchClient(URL, APIKey string) *MeilisearchClient {
	if APIKey != "" {
		return &MeilisearchClient{
			client: meilisearch.New(URL, meilisearch.WithAPIKey(APIKey)),
		}
	}
	return &MeilisearchClient{
		client: meilisearch.New(URL),
	}
}

// GetIndex returns a MeilisearchIndex instance for the given index name. GetIndex is the implementation for the
// Client interface.
func (c MeilisearchClient) GetIndex(name string) (Indexer, error) {
	index := c.client.Index(name)
	return &MeilisearchIndex{index: index}, nil
}

// MeilisearchIndex is the implementation of the Indexer interface for Meilisearch.
type MeilisearchIndex struct {
	index meilisearch.IndexManager
}

// IndexDocuments implements the Indexer interface for Meilisearch. It indexes the given documents. The documents
// must be a []map[string]any, where each document is a map[string]any of the document fields.
func (i *MeilisearchIndex) IndexDocuments(docs any) error {
	_, err := i.index.AddDocuments(docs, nil)
	return err
}
