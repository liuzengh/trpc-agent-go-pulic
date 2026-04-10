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
	"errors"

	"trpc.group/trpc-go/trpc-agent-go/internal/retrieval"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/document"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"
)

var errNilPipeline = errors.New("knowledge search: pipeline is nil")

// Result is the internal knowledge retrieval result.
type Result struct {
	Document *document.Document
	Score    float64
}

// HitsFromResults adapts knowledge results into retrieval hits.
func HitsFromResults(results []*Result) []retrieval.Hit[*document.Document] {
	hits := make([]retrieval.Hit[*document.Document], 0, len(results))
	for _, result := range results {
		if result == nil || result.Document == nil {
			continue
		}
		hits = append(hits, retrieval.Hit[*document.Document]{
			ID:    result.Document.ID,
			Item:  result.Document,
			Score: result.Score,
			Rank:  len(hits),
		})
	}
	return hits
}

// ResultsFromHits adapts hits back into knowledge results.
func ResultsFromHits(hits []retrieval.Hit[*document.Document]) []*Result {
	results := make([]*Result, 0, len(hits))
	for _, hit := range hits {
		if hit.Item == nil {
			continue
		}
		results = append(results, &Result{
			Document: hit.Item,
			Score:    hit.Score,
		})
	}
	return results
}

// ResultsFromSearchResult adapts vectorstore search results into knowledge results.
func ResultsFromSearchResult(searchResults *vectorstore.SearchResult) []*Result {
	if searchResults == nil {
		return nil
	}
	results := make([]*Result, 0, len(searchResults.Results))
	for _, scored := range searchResults.Results {
		if scored == nil {
			continue
		}
		results = append(results, &Result{
			Document: scored.Document,
			Score:    scored.Score,
		})
	}
	return results
}

// HitsFromSearchResult adapts vectorstore search results into retrieval hits.
func HitsFromSearchResult(searchResults *vectorstore.SearchResult) []retrieval.Hit[*document.Document] {
	return HitsFromResults(ResultsFromSearchResult(searchResults))
}

// Run executes a knowledge retrieval pipeline and returns results.
func Run(
	ctx context.Context,
	pipeline retrieval.Pipeline[Request, *document.Document],
	req Request,
) ([]*Result, error) {
	if pipeline == nil {
		return nil, errNilPipeline
	}
	hits, err := pipeline.Run(ctx, req)
	if err != nil {
		return nil, err
	}
	return ResultsFromHits(hits), nil
}

// NewBranch builds the default knowledge retrieval branch.
func NewBranch(
	rewrite retrieval.Rewriter[Request],
	channel retrieval.Channel[Request, *document.Document],
	rerank retrieval.Reranker[Request, *document.Document],
	post ...retrieval.Postprocessor[Request, *document.Document],
) *retrieval.Branch[Request, *document.Document] {
	return &retrieval.Branch[Request, *document.Document]{
		Rewrite: rewrite,
		Recall:  channel,
		Rerank:  rerank,
		Post:    append([]retrieval.Postprocessor[Request, *document.Document](nil), post...),
	}
}
