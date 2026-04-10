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
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/internal/retrieval"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

// RRFFusion fuses session dense and sparse search results with Reciprocal Rank Fusion.
type RRFFusion struct {
	K int
}

// Fuse merges dense and sparse session event results.
func (f RRFFusion) Fuse(
	_ context.Context,
	inputs []retrieval.NamedHits[session.EventSearchResult],
) ([]retrieval.Hit[session.EventSearchResult], error) {
	switch len(inputs) {
	case 0:
		return []retrieval.Hit[session.EventSearchResult]{}, nil
	case 1:
		return append([]retrieval.Hit[session.EventSearchResult](nil), inputs[0].Hits...), nil
	}

	k := f.K
	if k <= 0 {
		k = 60
	}

	type fusedEntry struct {
		result session.EventSearchResult
		score  float64
	}

	merged := make(map[string]*fusedEntry, len(inputs[0].Hits)+len(inputs[1].Hits))
	for _, input := range inputs {
		dense := input.Name == "dense"
		for rank, hit := range input.Hits {
			result := hit.Item
			id := hit.ID
			if id == "" {
				id = eventSearchResultID(result)
			}
			if id == "" {
				continue
			}
			rrfScore := 1.0 / float64(k+rank+1)
			if existing, ok := merged[id]; ok {
				existing.score += rrfScore
				if dense && existing.result.DenseScore == 0 {
					existing.result.DenseScore = result.DenseScore
				}
				if !dense && existing.result.SparseScore == 0 {
					existing.result.SparseScore = result.SparseScore
				}
				if strings.TrimSpace(existing.result.Text) == "" {
					existing.result.Text = result.Text
				}
				if existing.result.Role == "" {
					existing.result.Role = result.Role
				}
				continue
			}
			merged[id] = &fusedEntry{
				result: result,
				score:  rrfScore,
			}
		}
	}

	fused := make([]*fusedEntry, 0, len(merged))
	for _, entry := range merged {
		entry.result.Score = entry.score
		fused = append(fused, entry)
	}
	sort.Slice(fused, func(i, j int) bool {
		if fused[i].score == fused[j].score {
			return fused[i].result.EventCreatedAt.After(fused[j].result.EventCreatedAt)
		}
		return fused[i].score > fused[j].score
	})

	results := make([]session.EventSearchResult, 0, len(fused))
	for _, entry := range fused {
		results = append(results, entry.result)
	}
	return HitsFromResults(results), nil
}
