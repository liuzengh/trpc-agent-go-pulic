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
	"trpc.group/trpc-go/trpc-agent-go/memory"
)

// KindFallbackPolicy triggers a kind-less retry when kind-filtered results are
// too small.
type KindFallbackPolicy struct {
	MinResults int
}

// ShouldFallback decides whether kind fallback should run.
func (p KindFallbackPolicy) ShouldFallback(
	_ context.Context,
	req Request,
	primary []retrieval.Hit[*memory.Entry],
) (Request, bool, error) {
	if req.Options.Kind == "" || !req.Options.KindFallback {
		return req, false, nil
	}
	minResults := p.MinResults
	if minResults <= 0 {
		minResults = 1
	}
	if len(primary) >= minResults {
		return req, false, nil
	}
	fallbackOptions := req.Options
	fallbackOptions.Kind = ""
	fallbackOptions.KindFallback = false
	return req.WithOptions(fallbackOptions), true, nil
}

// KindFallbackMerger merges kind-constrained and fallback result sets.
type KindFallbackMerger struct {
	MaxResults int
	MergeFunc  func(
		primary []*memory.Entry,
		fallback []*memory.Entry,
		preferredKind memory.Kind,
		maxResults int,
	) []*memory.Entry
}

// Merge combines primary and fallback memory results.
func (m KindFallbackMerger) Merge(
	_ context.Context,
	req Request,
	primary []retrieval.Hit[*memory.Entry],
	fallback []retrieval.Hit[*memory.Entry],
) ([]retrieval.Hit[*memory.Entry], error) {
	mergeFunc := m.MergeFunc
	if mergeFunc == nil {
		mergeFunc = mergeKindFallbackEntries
	}
	merged := mergeFunc(
		EntriesFromHits(primary),
		EntriesFromHits(fallback),
		req.Options.Kind,
		m.MaxResults,
	)
	return HitsFromEntries(merged), nil
}

// RRFFusion fuses ranked lists with Reciprocal Rank Fusion.
type RRFFusion struct {
	K          int
	MaxResults int
	FuseFunc   func(
		primary []*memory.Entry,
		secondary []*memory.Entry,
		k int,
		maxResults int,
	) []*memory.Entry
}

// Fuse combines ranked memory result lists into one.
func (f RRFFusion) Fuse(
	_ context.Context,
	inputs []retrieval.NamedHits[*memory.Entry],
) ([]retrieval.Hit[*memory.Entry], error) {
	switch len(inputs) {
	case 0:
		return []retrieval.Hit[*memory.Entry]{}, nil
	case 1:
		return append([]retrieval.Hit[*memory.Entry](nil), inputs[0].Hits...), nil
	case 2:
		if f.FuseFunc != nil {
			merged := f.FuseFunc(
				EntriesFromHits(inputs[0].Hits),
				EntriesFromHits(inputs[1].Hits),
				f.resolveK(),
				f.MaxResults,
			)
			return HitsFromEntries(merged), nil
		}
	}
	return fuseRRF(inputs, f.resolveK(), f.MaxResults), nil
}

func (f RRFFusion) resolveK() int {
	if f.K > 0 {
		return f.K
	}
	return 60
}

func mergeKindFallbackEntries(
	primary []*memory.Entry,
	fallback []*memory.Entry,
	preferredKind memory.Kind,
	maxResults int,
) []*memory.Entry {
	seen := make(map[string]bool, len(primary))
	for _, entry := range primary {
		if entry == nil {
			continue
		}
		seen[entry.ID] = true
	}

	var kindMatch, kindOther []*memory.Entry
	for _, entry := range fallback {
		if entry == nil || seen[entry.ID] {
			continue
		}
		if effectiveKind(entry.Memory) == preferredKind {
			kindMatch = append(kindMatch, entry)
			continue
		}
		kindOther = append(kindOther, entry)
	}

	merged := make([]*memory.Entry, 0, len(primary)+len(kindMatch)+len(kindOther))
	merged = append(merged, primary...)
	merged = append(merged, kindMatch...)
	merged = append(merged, kindOther...)
	if maxResults > 0 && len(merged) > maxResults {
		merged = merged[:maxResults]
	}
	return merged
}

func fuseRRF(
	inputs []retrieval.NamedHits[*memory.Entry],
	k int,
	maxResults int,
) []retrieval.Hit[*memory.Entry] {
	type rrfEntry struct {
		entry *memory.Entry
		score float64
	}

	scores := make(map[string]*rrfEntry)
	for _, input := range inputs {
		for rank, hit := range input.Hits {
			entry := hit.Item
			if entry == nil {
				continue
			}
			id := hit.ID
			if id == "" {
				id = entry.ID
			}
			if id == "" {
				continue
			}
			score := 1.0 / float64(k+rank+1)
			if existing, ok := scores[id]; ok {
				existing.score += score
				continue
			}
			cloned := *entry
			scores[id] = &rrfEntry{
				entry: &cloned,
				score: score,
			}
		}
	}

	merged := make([]*memory.Entry, 0, len(scores))
	for _, scored := range scores {
		scored.entry.Score = scored.score
		merged = append(merged, scored.entry)
	}
	sortEntries(merged, false)
	if maxResults > 0 && len(merged) > maxResults {
		merged = merged[:maxResults]
	}
	return HitsFromEntries(merged)
}
