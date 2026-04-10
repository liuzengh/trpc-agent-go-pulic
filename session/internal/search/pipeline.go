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
	"encoding/json"
	"errors"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/internal/retrieval"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

var errNilPipeline = errors.New("session search: pipeline is nil")

// HitsFromResults adapts session search results into retrieval hits.
func HitsFromResults(results []session.EventSearchResult) []retrieval.Hit[session.EventSearchResult] {
	hits := make([]retrieval.Hit[session.EventSearchResult], 0, len(results))
	for _, result := range results {
		hits = append(hits, retrieval.Hit[session.EventSearchResult]{
			ID:    eventSearchResultID(result),
			Item:  result,
			Score: result.Score,
			Rank:  len(hits),
		})
	}
	return hits
}

// ResultsFromHits adapts hits back into session search results.
func ResultsFromHits(hits []retrieval.Hit[session.EventSearchResult]) []session.EventSearchResult {
	results := make([]session.EventSearchResult, 0, len(hits))
	for _, hit := range hits {
		result := hit.Item
		result.Score = hit.Score
		results = append(results, result)
	}
	return results
}

// Run executes a session retrieval pipeline and returns event search results.
func Run(
	ctx context.Context,
	pipeline retrieval.Pipeline[Request, session.EventSearchResult],
	req Request,
) ([]session.EventSearchResult, error) {
	if pipeline == nil {
		return nil, errNilPipeline
	}
	hits, err := pipeline.Run(ctx, req)
	if err != nil {
		return nil, err
	}
	return ResultsFromHits(hits), nil
}

// NewDenseBranch builds the dense branch for session event search.
func NewDenseBranch(
	channel retrieval.Channel[Request, session.EventSearchResult],
	post ...retrieval.Postprocessor[Request, session.EventSearchResult],
) *retrieval.Branch[Request, session.EventSearchResult] {
	return &retrieval.Branch[Request, session.EventSearchResult]{
		Recall: channel,
		Post:   append([]retrieval.Postprocessor[Request, session.EventSearchResult](nil), post...),
	}
}

// NewKeywordBranch builds the keyword branch for session event search.
func NewKeywordBranch(
	channel retrieval.Channel[Request, session.EventSearchResult],
	post ...retrieval.Postprocessor[Request, session.EventSearchResult],
) *retrieval.Branch[Request, session.EventSearchResult] {
	return &retrieval.Branch[Request, session.EventSearchResult]{
		Recall: channel,
		Post:   append([]retrieval.Postprocessor[Request, session.EventSearchResult](nil), post...),
	}
}

// NewHybridPipeline builds a hybrid session event retrieval pipeline.
func NewHybridPipeline(
	pipelines []retrieval.NamedPipeline[Request, session.EventSearchResult],
	fusion retrieval.Fusion[session.EventSearchResult],
	post ...retrieval.Postprocessor[Request, session.EventSearchResult],
) *retrieval.HybridPipeline[Request, session.EventSearchResult] {
	return &retrieval.HybridPipeline[Request, session.EventSearchResult]{
		Pipelines: append([]retrieval.NamedPipeline[Request, session.EventSearchResult](nil), pipelines...),
		Fusion:    fusion,
		Post:      append([]retrieval.Postprocessor[Request, session.EventSearchResult](nil), post...),
	}
}

func eventSearchResultID(result session.EventSearchResult) string {
	keyParts := []string{
		result.SessionKey.AppName,
		result.SessionKey.UserID,
		result.SessionKey.SessionID,
	}
	if id := strings.TrimSpace(result.Event.ID); id != "" {
		return strings.Join(append(keyParts, id), "|")
	}
	if eventBytes, err := json.Marshal(result.Event); err == nil {
		return strings.Join(
			append(keyParts, string(eventBytes)),
			"|",
		)
	}
	return strings.Join(
		append(keyParts,
			result.EventCreatedAt.UTC().Format(time.RFC3339Nano),
			strings.TrimSpace(result.Role.String()),
			strings.TrimSpace(result.Text),
		),
		"|",
	)
}
