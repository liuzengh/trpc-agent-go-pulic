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
	"trpc.group/trpc-go/trpc-agent-go/knowledge/reranker"
)

// RerankerAdapter adapts knowledge reranker into the retrieval core.
type RerankerAdapter struct {
	Reranker reranker.Reranker
}

// Rerank reorders knowledge hits.
func (r RerankerAdapter) Rerank(
	ctx context.Context,
	req Request,
	hits []retrieval.Hit[*document.Document],
) ([]retrieval.Hit[*document.Document], error) {
	results := make([]*reranker.Result, 0, len(hits))
	for _, hit := range hits {
		if hit.Item == nil {
			continue
		}
		results = append(results, &reranker.Result{
			Document: hit.Item,
			Score:    hit.Score,
		})
	}
	reranked, err := r.Reranker.Rerank(ctx, &reranker.Query{
		Text:       req.Text,
		FinalQuery: req.FinalQuery,
		History:    req.History,
		UserID:     req.UserID,
		SessionID:  req.SessionID,
	}, results)
	if err != nil {
		return nil, err
	}
	out := make([]*Result, 0, len(reranked))
	for _, result := range reranked {
		if result == nil {
			continue
		}
		out = append(out, &Result{
			Document: result.Document,
			Score:    result.Score,
		})
	}
	return HitsFromResults(out), nil
}
