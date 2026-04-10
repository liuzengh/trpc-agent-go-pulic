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
	"fmt"

	"trpc.group/trpc-go/trpc-agent-go/internal/retrieval"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"
)

var (
	errNilSearchQuery = errors.New("knowledge vectorstore search: query cannot be nil")
	errNilPipeline    = errors.New("knowledge vectorstore search: pipeline is nil")
)

// HitsFromSearchResult adapts vectorstore results into retrieval hits.
func HitsFromSearchResult(
	result *vectorstore.SearchResult,
) []retrieval.Hit[*vectorstore.ScoredDocument] {
	if result == nil {
		return nil
	}
	hits := make([]retrieval.Hit[*vectorstore.ScoredDocument], 0, len(result.Results))
	for _, doc := range result.Results {
		if doc == nil {
			continue
		}
		id := ""
		if doc.Document != nil {
			id = doc.Document.ID
		}
		hits = append(hits, retrieval.Hit[*vectorstore.ScoredDocument]{
			ID:    id,
			Item:  doc,
			Score: doc.Score,
			Rank:  len(hits),
		})
	}
	return hits
}

// SearchResultFromHits adapts retrieval hits back into vectorstore results.
func SearchResultFromHits(
	hits []retrieval.Hit[*vectorstore.ScoredDocument],
) *vectorstore.SearchResult {
	result := &vectorstore.SearchResult{
		Results: make([]*vectorstore.ScoredDocument, 0, len(hits)),
	}
	for _, hit := range hits {
		if hit.Item == nil {
			continue
		}
		scored := *hit.Item
		scored.Score = hit.Score
		result.Results = append(result.Results, &scored)
	}
	return result
}

// Run executes a knowledge vectorstore pipeline and returns a SearchResult.
func Run(
	ctx context.Context,
	pipeline retrieval.Pipeline[Request, *vectorstore.ScoredDocument],
	query *vectorstore.SearchQuery,
) (*vectorstore.SearchResult, error) {
	if query == nil {
		return nil, errNilSearchQuery
	}
	if pipeline == nil {
		return nil, errNilPipeline
	}
	hits, err := pipeline.Run(ctx, Request{Query: query})
	if err != nil {
		return nil, err
	}
	return SearchResultFromHits(hits), nil
}

// NewVectorBranch builds the vector branch for vectorstore search.
func NewVectorBranch(
	channel retrieval.Channel[Request, *vectorstore.ScoredDocument],
	post ...retrieval.Postprocessor[Request, *vectorstore.ScoredDocument],
) *retrieval.Branch[Request, *vectorstore.ScoredDocument] {
	return &retrieval.Branch[Request, *vectorstore.ScoredDocument]{
		Recall: channel,
		Post:   append([]retrieval.Postprocessor[Request, *vectorstore.ScoredDocument](nil), post...),
	}
}

// NewKeywordBranch builds the keyword branch for vectorstore search.
func NewKeywordBranch(
	channel retrieval.Channel[Request, *vectorstore.ScoredDocument],
	post ...retrieval.Postprocessor[Request, *vectorstore.ScoredDocument],
) *retrieval.Branch[Request, *vectorstore.ScoredDocument] {
	return &retrieval.Branch[Request, *vectorstore.ScoredDocument]{
		Recall: channel,
		Post:   append([]retrieval.Postprocessor[Request, *vectorstore.ScoredDocument](nil), post...),
	}
}

// NewHybridBranch builds the hybrid branch for vectorstore search.
func NewHybridBranch(
	channel retrieval.Channel[Request, *vectorstore.ScoredDocument],
	post ...retrieval.Postprocessor[Request, *vectorstore.ScoredDocument],
) *retrieval.Branch[Request, *vectorstore.ScoredDocument] {
	return &retrieval.Branch[Request, *vectorstore.ScoredDocument]{
		Recall: channel,
		Post:   append([]retrieval.Postprocessor[Request, *vectorstore.ScoredDocument](nil), post...),
	}
}

// NewFilterBranch builds the filter branch for vectorstore search.
func NewFilterBranch(
	channel retrieval.Channel[Request, *vectorstore.ScoredDocument],
	post ...retrieval.Postprocessor[Request, *vectorstore.ScoredDocument],
) *retrieval.Branch[Request, *vectorstore.ScoredDocument] {
	return &retrieval.Branch[Request, *vectorstore.ScoredDocument]{
		Recall: channel,
		Post:   append([]retrieval.Postprocessor[Request, *vectorstore.ScoredDocument](nil), post...),
	}
}

// ModePipeline routes a SearchQuery to a mode-specific retrieval pipeline.
type ModePipeline struct {
	Vector      retrieval.Pipeline[Request, *vectorstore.ScoredDocument]
	Keyword     retrieval.Pipeline[Request, *vectorstore.ScoredDocument]
	Hybrid      retrieval.Pipeline[Request, *vectorstore.ScoredDocument]
	Filter      retrieval.Pipeline[Request, *vectorstore.ScoredDocument]
	Default     retrieval.Pipeline[Request, *vectorstore.ScoredDocument]
	InvalidMode func(mode vectorstore.SearchMode) error
}

// Run executes the pipeline that matches the request search mode.
func (p *ModePipeline) Run(
	ctx context.Context,
	req Request,
) ([]retrieval.Hit[*vectorstore.ScoredDocument], error) {
	if req.Query == nil {
		return nil, errNilSearchQuery
	}

	var selected retrieval.Pipeline[Request, *vectorstore.ScoredDocument]
	switch req.Query.SearchMode {
	case vectorstore.SearchModeVector:
		selected = p.Vector
	case vectorstore.SearchModeKeyword:
		selected = p.Keyword
	case vectorstore.SearchModeHybrid:
		selected = p.Hybrid
	case vectorstore.SearchModeFilter:
		selected = p.Filter
	default:
		if p.InvalidMode != nil {
			return nil, p.InvalidMode(req.Query.SearchMode)
		}
		selected = p.Default
	}
	if selected == nil {
		return nil, fmt.Errorf("knowledge vectorstore search: pipeline not configured for mode %d", req.Query.SearchMode)
	}
	return selected.Run(ctx, req)
}
