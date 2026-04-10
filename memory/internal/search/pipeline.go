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
	"trpc.group/trpc-go/trpc-agent-go/memory"
)

var errNilPipeline = errors.New("memory search: pipeline is nil")

// HitsFromEntries adapts scored memory entries into retrieval hits.
func HitsFromEntries(entries []*memory.Entry) []retrieval.Hit[*memory.Entry] {
	hits := make([]retrieval.Hit[*memory.Entry], 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		hits = append(hits, retrieval.Hit[*memory.Entry]{
			ID:    entry.ID,
			Item:  entry,
			Score: entry.Score,
			Rank:  len(hits),
		})
	}
	return hits
}

// EntriesFromHits adapts hits back into scored memory entries.
func EntriesFromHits(hits []retrieval.Hit[*memory.Entry]) []*memory.Entry {
	entries := make([]*memory.Entry, 0, len(hits))
	for _, hit := range hits {
		if hit.Item == nil {
			continue
		}
		entry := hit.Item
		entry.Score = hit.Score
		entries = append(entries, entry)
	}
	return entries
}

// Run executes a memory retrieval pipeline and returns memory entries.
func Run(
	ctx context.Context,
	pipeline retrieval.Pipeline[Request, *memory.Entry],
	req Request,
) ([]*memory.Entry, error) {
	if pipeline == nil {
		return nil, errNilPipeline
	}
	hits, err := pipeline.Run(ctx, req)
	if err != nil {
		return nil, err
	}
	return EntriesFromHits(hits), nil
}

// NewDenseBranch builds the dense branch for memory search.
func NewDenseBranch(
	channel retrieval.Channel[Request, *memory.Entry],
	post ...retrieval.Postprocessor[Request, *memory.Entry],
) *retrieval.Branch[Request, *memory.Entry] {
	return &retrieval.Branch[Request, *memory.Entry]{
		Recall: channel,
		Post:   append([]retrieval.Postprocessor[Request, *memory.Entry](nil), post...),
	}
}

// NewKeywordBranch builds the keyword branch for memory search.
func NewKeywordBranch(
	channel retrieval.Channel[Request, *memory.Entry],
	post ...retrieval.Postprocessor[Request, *memory.Entry],
) *retrieval.Branch[Request, *memory.Entry] {
	return &retrieval.Branch[Request, *memory.Entry]{
		Recall: channel,
		Post:   append([]retrieval.Postprocessor[Request, *memory.Entry](nil), post...),
	}
}

// NewKindFallbackPipeline builds a kind fallback pipeline.
func NewKindFallbackPipeline(
	primary retrieval.Pipeline[Request, *memory.Entry],
	fallback retrieval.Pipeline[Request, *memory.Entry],
	policy retrieval.FallbackPolicy[Request, *memory.Entry],
	merger retrieval.Merger[Request, *memory.Entry],
	post ...retrieval.Postprocessor[Request, *memory.Entry],
) *retrieval.FallbackPipeline[Request, *memory.Entry] {
	return &retrieval.FallbackPipeline[Request, *memory.Entry]{
		Primary:  primary,
		Policy:   policy,
		Fallback: fallback,
		Merge:    merger,
		Post:     append([]retrieval.Postprocessor[Request, *memory.Entry](nil), post...),
	}
}

// NewHybridPipeline builds a hybrid pipeline over ordered named pipelines.
func NewHybridPipeline(
	pipelines []retrieval.NamedPipeline[Request, *memory.Entry],
	fusion retrieval.Fusion[*memory.Entry],
	post ...retrieval.Postprocessor[Request, *memory.Entry],
) *retrieval.HybridPipeline[Request, *memory.Entry] {
	return &retrieval.HybridPipeline[Request, *memory.Entry]{
		Pipelines: append([]retrieval.NamedPipeline[Request, *memory.Entry](nil), pipelines...),
		Fusion:    fusion,
		Post:      append([]retrieval.Postprocessor[Request, *memory.Entry](nil), post...),
	}
}
