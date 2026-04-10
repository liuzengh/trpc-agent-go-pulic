//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//

package search

import (
	"context"

	"trpc.group/trpc-go/trpc-agent-go/internal/retrieval"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/document"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"
)

// VectorStoreChannel adapts vectorstore search into a retrieval channel.
type VectorStoreChannel struct {
	VectorStore vectorstore.VectorStore
}

// Recall executes vector store search.
func (c VectorStoreChannel) Recall(
	ctx context.Context,
	req Request,
) ([]retrieval.Hit[*document.Document], error) {
	searchResults, err := c.VectorStore.Search(ctx, req.SearchQuery())
	if err != nil {
		return nil, err
	}
	return HitsFromSearchResult(searchResults), nil
}
