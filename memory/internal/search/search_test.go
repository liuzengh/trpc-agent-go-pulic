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
	"math"
	"reflect"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/internal/retrieval"
	"trpc.group/trpc-go/trpc-agent-go/memory"
)

func TestKindFallbackPolicy(t *testing.T) {
	req := Request{
		Options: memory.SearchOptions{
			Kind:         memory.KindEpisode,
			KindFallback: true,
		},
	}

	gotReq, ok, err := (KindFallbackPolicy{MinResults: 2}).ShouldFallback(
		context.Background(),
		req,
		[]retrieval.Hit[*memory.Entry]{{ID: "1"}},
	)
	if err != nil {
		t.Fatalf("ShouldFallback() error = %v", err)
	}
	if !ok {
		t.Fatalf("ShouldFallback() ok = false, want true")
	}
	if gotReq.Options.Kind != "" || gotReq.Options.KindFallback {
		t.Fatalf("fallback request = %#v, want kind cleared", gotReq.Options)
	}

	_, ok, err = (KindFallbackPolicy{MinResults: 1}).ShouldFallback(
		context.Background(),
		req,
		[]retrieval.Hit[*memory.Entry]{{ID: "1"}},
	)
	if err != nil {
		t.Fatalf("ShouldFallback() error = %v", err)
	}
	if ok {
		t.Fatalf("ShouldFallback() ok = true, want false")
	}
}

func TestKindFallbackMergerPreferredKindOrder(t *testing.T) {
	primary := HitsFromEntries([]*memory.Entry{
		newEntry("p1", "primary", memory.KindEpisode, 0.9),
	})
	fallback := HitsFromEntries([]*memory.Entry{
		newEntry("f1", "fact", memory.KindFact, 0.8),
		newEntry("f2", "episode", memory.KindEpisode, 0.7),
	})

	got, err := (KindFallbackMerger{MaxResults: 10}).Merge(
		context.Background(),
		Request{Options: memory.SearchOptions{Kind: memory.KindEpisode}},
		primary,
		fallback,
	)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	ids := []string{got[0].ID, got[1].ID, got[2].ID}
	if want := []string{"p1", "f2", "f1"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestRRFFusionDedupAndRankAccumulation(t *testing.T) {
	inputs := []retrieval.NamedHits[*memory.Entry]{
		{
			Name: "dense",
			Hits: HitsFromEntries([]*memory.Entry{
				newEntry("a", "alpha", memory.KindFact, 0.9),
				newEntry("b", "beta", memory.KindFact, 0.8),
			}),
		},
		{
			Name: "keyword",
			Hits: HitsFromEntries([]*memory.Entry{
				newEntry("b", "beta", memory.KindFact, 0.7),
				newEntry("c", "gamma", memory.KindFact, 0.6),
			}),
		},
	}

	got, err := (RRFFusion{K: 60, MaxResults: 10}).Fuse(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Fuse() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(Fuse()) = %d, want 3", len(got))
	}
	if got[0].ID != "b" {
		t.Fatalf("top id = %q, want %q", got[0].ID, "b")
	}
	wantScore := 1.0/62.0 + 1.0/61.0
	if math.Abs(got[0].Score-wantScore) > 1e-9 {
		t.Fatalf("top score = %v, want %v", got[0].Score, wantScore)
	}
}

func TestPostprocessorsPreserveOrdering(t *testing.T) {
	baseTime := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	req := Request{
		Options: memory.SearchOptions{
			Deduplicate: true,
		},
	}

	sortHits, err := (SortPostprocessor{OrderByEventTime: true}).Postprocess(
		context.Background(),
		req,
		HitsFromEntries([]*memory.Entry{
			newTimedEntry("late", "late", memory.KindFact, 0.9, baseTime.Add(2*time.Hour)),
			newTimedEntry("early", "early", memory.KindFact, 0.9, baseTime),
		}),
	)
	if err != nil {
		t.Fatalf("SortPostprocessor error = %v", err)
	}
	if got := []string{sortHits[0].ID, sortHits[1].ID}; !reflect.DeepEqual(got, []string{"early", "late"}) {
		t.Fatalf("sorted ids = %v, want [early late]", got)
	}

	kindHits, err := (KindPrioritySortPostprocessor{
		PreferredKind:    memory.KindEpisode,
		OrderByEventTime: false,
	}).Postprocess(
		context.Background(),
		req,
		HitsFromEntries([]*memory.Entry{
			newEntry("fact", "fact", memory.KindFact, 0.9),
			newEntry("episode", "episode", memory.KindEpisode, 0.8),
		}),
	)
	if err != nil {
		t.Fatalf("KindPrioritySortPostprocessor error = %v", err)
	}
	if got := []string{kindHits[0].ID, kindHits[1].ID}; !reflect.DeepEqual(got, []string{"episode", "fact"}) {
		t.Fatalf("kind-priority ids = %v, want [episode fact]", got)
	}

	dedupHits, err := (DeduplicatePostprocessor{
		DeduplicateFunc: func(results []*memory.Entry) []*memory.Entry {
			return results[:1]
		},
	}).Postprocess(
		context.Background(),
		req,
		HitsFromEntries([]*memory.Entry{
			newEntry("a", "same", memory.KindFact, 0.9),
			newEntry("b", "same", memory.KindFact, 0.8),
		}),
	)
	if err != nil {
		t.Fatalf("DeduplicatePostprocessor error = %v", err)
	}
	if len(dedupHits) != 1 || dedupHits[0].ID != "a" {
		t.Fatalf("dedup result = %#v, want only a", dedupHits)
	}

	topKHits, err := (TopKPostprocessor{MaxResults: 1}).Postprocess(
		context.Background(),
		req,
		HitsFromEntries([]*memory.Entry{
			newEntry("a", "a", memory.KindFact, 0.9),
			newEntry("b", "b", memory.KindFact, 0.8),
		}),
	)
	if err != nil {
		t.Fatalf("TopKPostprocessor error = %v", err)
	}
	if len(topKHits) != 1 || topKHits[0].ID != "a" {
		t.Fatalf("top-k result = %#v, want only a", topKHits)
	}
}

func newEntry(id, text string, kind memory.Kind, score float64) *memory.Entry {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	return &memory.Entry{
		ID:        id,
		AppName:   "app",
		UserID:    "user",
		CreatedAt: now,
		UpdatedAt: now,
		Score:     score,
		Memory: &memory.Memory{
			Memory: text,
			Kind:   kind,
		},
	}
}

func newTimedEntry(
	id string,
	text string,
	kind memory.Kind,
	score float64,
	eventTime time.Time,
) *memory.Entry {
	entry := newEntry(id, text, kind, score)
	entry.Memory.EventTime = &eventTime
	return entry
}
