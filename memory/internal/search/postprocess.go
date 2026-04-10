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
	"sort"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/internal/retrieval"
	"trpc.group/trpc-go/trpc-agent-go/memory"
)

// SortPostprocessor applies the standard memory result ordering.
type SortPostprocessor struct {
	OrderByEventTime bool
}

// Postprocess sorts hits by score and tie-break rules.
func (p SortPostprocessor) Postprocess(
	_ context.Context,
	_ Request,
	hits []retrieval.Hit[*memory.Entry],
) ([]retrieval.Hit[*memory.Entry], error) {
	entries := EntriesFromHits(hits)
	if len(entries) < 2 {
		return HitsFromEntries(entries), nil
	}
	sortEntries(entries, p.OrderByEventTime)
	return HitsFromEntries(entries), nil
}

// KindPrioritySortPostprocessor sorts preferred-kind entries ahead of fallback
// entries while preserving in-group ordering rules.
type KindPrioritySortPostprocessor struct {
	PreferredKind    memory.Kind
	OrderByEventTime bool
}

// Postprocess sorts preferred-kind and fallback groups separately.
func (p KindPrioritySortPostprocessor) Postprocess(
	_ context.Context,
	_ Request,
	hits []retrieval.Hit[*memory.Entry],
) ([]retrieval.Hit[*memory.Entry], error) {
	entries := EntriesFromHits(hits)
	if len(entries) < 2 {
		return HitsFromEntries(entries), nil
	}

	preferredKind := p.PreferredKind
	if preferredKind == "" {
		sortEntries(entries, p.OrderByEventTime)
		return HitsFromEntries(entries), nil
	}

	preferred := make([]*memory.Entry, 0, len(entries))
	fallback := make([]*memory.Entry, 0, len(entries))
	for _, entry := range entries {
		if entry != nil && effectiveKind(entry.Memory) == preferredKind {
			preferred = append(preferred, entry)
			continue
		}
		fallback = append(fallback, entry)
	}
	sortEntries(preferred, p.OrderByEventTime)
	sortEntries(fallback, p.OrderByEventTime)
	merged := make([]*memory.Entry, 0, len(entries))
	merged = append(merged, preferred...)
	merged = append(merged, fallback...)
	return HitsFromEntries(merged), nil
}

// DeduplicatePostprocessor removes near-duplicate results.
type DeduplicatePostprocessor struct {
	DeduplicateFunc func(results []*memory.Entry) []*memory.Entry
}

// Postprocess deduplicates hits when enabled.
func (p DeduplicatePostprocessor) Postprocess(
	_ context.Context,
	req Request,
	hits []retrieval.Hit[*memory.Entry],
) ([]retrieval.Hit[*memory.Entry], error) {
	if !req.Options.Deduplicate || len(hits) < 2 || p.DeduplicateFunc == nil {
		return hits, nil
	}
	return HitsFromEntries(p.DeduplicateFunc(EntriesFromHits(hits))), nil
}

// TopKPostprocessor truncates results to the configured limit.
type TopKPostprocessor struct {
	MaxResults int
}

// Postprocess keeps only the highest-ranked K results.
func (p TopKPostprocessor) Postprocess(
	_ context.Context,
	_ Request,
	hits []retrieval.Hit[*memory.Entry],
) ([]retrieval.Hit[*memory.Entry], error) {
	if p.MaxResults <= 0 || len(hits) <= p.MaxResults {
		return hits, nil
	}
	return append([]retrieval.Hit[*memory.Entry](nil), hits[:p.MaxResults]...), nil
}

// ThresholdPostprocessor filters out low-scoring results.
type ThresholdPostprocessor struct {
	SimilarityThreshold float64
	Skip                bool
}

// Postprocess applies the similarity threshold unless skipped.
func (p ThresholdPostprocessor) Postprocess(
	_ context.Context,
	_ Request,
	hits []retrieval.Hit[*memory.Entry],
) ([]retrieval.Hit[*memory.Entry], error) {
	if p.Skip || p.SimilarityThreshold <= 0 || len(hits) == 0 {
		return hits, nil
	}
	filtered := make([]retrieval.Hit[*memory.Entry], 0, len(hits))
	for _, hit := range hits {
		if hit.Score >= p.SimilarityThreshold {
			filtered = append(filtered, hit)
		}
	}
	for i := range filtered {
		filtered[i].Rank = i
	}
	return filtered, nil
}

func sortEntries(entries []*memory.Entry, orderByEventTime bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		var leftScore float64
		if entries[i] != nil {
			leftScore = entries[i].Score
		}
		var rightScore float64
		if entries[j] != nil {
			rightScore = entries[j].Score
		}
		return lessEntry(entries[i], entries[j], leftScore, rightScore, orderByEventTime)
	})
}

func lessEntry(
	left *memory.Entry,
	right *memory.Entry,
	leftScore float64,
	rightScore float64,
	orderByEventTime bool,
) bool {
	switch {
	case left == nil && right == nil:
		return false
	case left == nil:
		return false
	case right == nil:
		return true
	}
	if leftScore != rightScore {
		return leftScore > rightScore
	}
	if orderByEventTime {
		leftTime := entryEventTime(left)
		rightTime := entryEventTime(right)
		switch {
		case leftTime == nil && rightTime != nil:
			return false
		case leftTime != nil && rightTime == nil:
			return true
		case leftTime != nil && rightTime != nil && !leftTime.Equal(*rightTime):
			return leftTime.Before(*rightTime)
		}
	}
	if !left.UpdatedAt.Equal(right.UpdatedAt) {
		return left.UpdatedAt.After(right.UpdatedAt)
	}
	if !left.CreatedAt.Equal(right.CreatedAt) {
		return left.CreatedAt.After(right.CreatedAt)
	}
	return left.ID < right.ID
}

func entryEventTime(entry *memory.Entry) *time.Time {
	if entry == nil || entry.Memory == nil {
		return nil
	}
	return entry.Memory.EventTime
}

func effectiveKind(mem *memory.Memory) memory.Kind {
	if mem == nil {
		return ""
	}
	if mem.Kind != "" {
		return mem.Kind
	}
	return memory.KindFact
}
